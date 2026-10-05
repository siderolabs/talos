// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// stopModeControllerName is also the owner the power API stamps on the records it writes, so that
// this controller is allowed to destroy them.
const stopModeControllerName = hypervisor.VirtualMachineStopModeOwner

// VirtualMachineStopModeController retires the records of stops which are over.
//
// A stop mode is a parameter of a transition, written by the power API and read by the controllers
// which carry the stop out. Nothing else writes one, and this controller never does: it only
// destroys, which is what gives a record owned by an API call a lifetime bounded by the stop it
// describes rather than by the uptime of the host.
//
// A record is kept while its virtual machine is driven towards running and the stop it describes
// has not been seen driven yet: a stop is recorded before the machine configuration which drives it
// is patched, so retiring the records of running machines would race that patch and quietly turn
// every graceful stop into a destroy.
//
// Once its stop has been seen driven, a record is retired when either the domain is gone, which
// VirtualMachineController attests by releasing its claim on the domain spec, or the virtual machine
// is driven towards running again, by whatever path: a stop which was not finished by then is
// abandoned, and what it asked for must not decide how some later stop is carried out.
//
// The domain is read as gone only on that release, never on a domain status which has not been
// published, so a graceful stop in flight is not retired while its domain may still be running.
type VirtualMachineStopModeController struct {
	// engaged holds the version of each record seen while its virtual machine was driven towards
	// stopped. A record written again since, by a later stop, is a stop which has not been driven.
	engaged map[resource.ID]resource.Version
}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineStopModeController) Name() string {
	return stopModeControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineStopModeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineStopModeType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineStopModeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineStopModeType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineStopModeController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	if ctrl.engaged == nil {
		ctrl.engaged = map[resource.ID]resource.Version{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineStopModeController) reconcile(ctx context.Context, r controller.ReaderWriter) error {
	stopModes, err := safe.ReaderListAll[*hypervisor.VirtualMachineStopMode](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine stop modes: %w", err)
	}

	ctrl.forgetRetired(stopModes)

	if stopModes.Len() == 0 {
		return nil
	}

	running, err := drivenTowardsRunning(ctx, r)
	if err != nil {
		return err
	}

	claimed, err := claimedDomains(ctx, r)
	if err != nil {
		return err
	}

	var errs []error

	for stopMode := range stopModes.All() {
		md := stopMode.Metadata()
		name := md.ID()

		if !ctrl.over(name, md.Version(), running, claimed) {
			continue
		}

		if err := r.Destroy(ctx, md); err != nil && !state.IsNotFoundError(err) {
			errs = append(errs, fmt.Errorf("failed to retire the stop mode of virtual machine %q: %w", name, err))

			continue
		}

		delete(ctrl.engaged, name)
	}

	return errors.Join(errs...)
}

// forgetRetired drops the engagement of every record which is no longer there.
func (ctrl *VirtualMachineStopModeController) forgetRetired(stopModes safe.List[*hypervisor.VirtualMachineStopMode]) {
	recorded := make(map[resource.ID]struct{}, stopModes.Len())
	for stopMode := range stopModes.All() {
		recorded[stopMode.Metadata().ID()] = struct{}{}
	}

	for name := range ctrl.engaged {
		if _, ok := recorded[name]; !ok {
			delete(ctrl.engaged, name)
		}
	}
}

// over decides whether the stop a record describes is over.
func (ctrl *VirtualMachineStopModeController) over(
	name resource.ID, version resource.Version, running, claimed map[resource.ID]struct{},
) bool {
	engagedVersion, seen := ctrl.engaged[name]
	engaged := seen && engagedVersion.Equal(version)

	if _, ok := running[name]; ok {
		// Not driven yet, or driven and since abandoned by a start.
		return engaged
	}

	ctrl.engaged[name] = version

	_, held := claimed[name]

	return !held
}

// drivenTowardsRunning names the virtual machines the machine configuration drives towards running.
func drivenTowardsRunning(ctx context.Context, r controller.Reader) (map[resource.ID]struct{}, error) {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual machine specs: %w", err)
	}

	names := make(map[resource.ID]struct{}, specs.Len())

	for spec := range specs.All() {
		if spec.TypedSpec().PowerState == hypervisorhelpers.PowerStateRunning.String() {
			names[spec.Metadata().ID()] = struct{}{}
		}
	}

	return names, nil
}

// claimedDomains names the virtual machines whose domain spec VirtualMachineController still holds,
// which it does for as long as it may have a domain of that virtual machine left to stop.
func claimedDomains(ctx context.Context, r controller.Reader) (map[resource.ID]struct{}, error) {
	domainSpecs, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual machine domain specs: %w", err)
	}

	owner := (&VirtualMachineController{}).Name()
	names := make(map[resource.ID]struct{}, domainSpecs.Len())

	for domainSpec := range domainSpecs.All() {
		if domainSpec.Metadata().Finalizers().Has(owner) {
			names[domainSpec.Metadata().ID()] = struct{}{}
		}
	}

	return names, nil
}
