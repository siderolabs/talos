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
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// CloudInitSpecController projects seed intent from all VM specs, including external producers.
type CloudInitSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *CloudInitSpecController) Name() string {
	return "hypervisor.CloudInitSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CloudInitSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CloudInitSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.CloudInitSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CloudInitSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *CloudInitSpecController) reconcile(ctx context.Context, r controller.ReaderWriter) error {
	vms, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("list virtual machine specs: %w", err)
	}

	wanted := map[resource.ID]struct{}{}

	var errs []error

	for vm := range vms.All() {
		if vm.Metadata().Phase() != resource.PhaseRunning || vm.TypedSpec().CloudInit == nil {
			continue
		}

		id := vm.Metadata().ID()
		wanted[id] = struct{}{}
		cloud := vm.TypedSpec().CloudInit

		if err := safe.WriterModify(ctx, r, hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, id), func(out *hypervisor.CloudInitSpec) error {
			*out.TypedSpec() = hypervisor.CloudInitSpecSpec{
				Library:       cloud.Library,
				MetaData:      cloud.MetaData,
				UserData:      cloud.UserData,
				NetworkConfig: cloud.NetworkConfig,
			}

			return nil
		}); err != nil {
			errs = append(errs, fmt.Errorf("project cloud-init for %q: %w", id, err))
		}
	}

	return errors.Join(append(errs, cleanupOutputs[*hypervisor.CloudInitSpec](ctx, r, "cloud-init spec", wanted))...)
}
