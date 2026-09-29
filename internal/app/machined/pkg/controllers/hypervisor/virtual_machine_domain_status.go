// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	libvirt "github.com/digitalocean/go-libvirt"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// VirtualMachineDomainStatusController inventories all libvirt domains, regardless of Talos ownership.
type VirtualMachineDomainStatusController struct {
	V1Alpha1Mode machineruntime.Mode
	Open         func(context.Context) (libvirtdomain.Client, error)
	Watch        func(context.Context) (<-chan struct{}, error)
}

// Name implements controller.Controller.
func (*VirtualMachineDomainStatusController) Name() string {
	return "hypervisor.VirtualMachineDomainStatusController"
}

// Inputs implements controller.Controller.
func (*VirtualMachineDomainStatusController) Inputs() []controller.Input { return nil }

// Outputs implements controller.Controller.
func (*VirtualMachineDomainStatusController) Outputs() []controller.Output {
	return []controller.Output{{Type: hypervisor.VirtualMachineDomainStatusType, Kind: controller.OutputExclusive}}
}

// Run implements controller.Controller.
func (ctrl *VirtualMachineDomainStatusController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	events, err := ctrl.Watch(watchCtx)
	if err != nil {
		return fmt.Errorf("watch libvirt domains: %w", err)
	}

	client, err := ctrl.Open(ctx)
	if err != nil {
		return fmt.Errorf("open libvirt: %w", err)
	}
	defer client.Close()

	// The watch is registered before the first inventory. Events arriving during
	// the inventory remain queued and cause a follow-up scan.
	if err = ctrl.reconcile(ctx, runtime, client); err != nil {
		return err
	}

	runtime.ResetRestartBackoff()

	return ctrl.watchEvents(ctx, runtime, client, events)
}

func (ctrl *VirtualMachineDomainStatusController) watchEvents(ctx context.Context, runtime controller.Runtime, client libvirtdomain.Client, events <-chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-events:
			if !ok {
				select {
				case <-ctx.Done():
					return nil
				default:
					return errors.New("libvirt domain watch closed")
				}
			}
		}

		if err := ctrl.reconcile(ctx, runtime, client); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDomainStatusController) reconcile(ctx context.Context, runtime controller.Runtime, client libvirtdomain.Client) error {
	runtime.StartTrackingOutputs()

	domains, err := client.Domains()
	if err != nil {
		return fmt.Errorf("list libvirt domains: %w", err)
	}

	var errs error

	for _, domain := range domains {
		status := observeDomain(client, domain)

		if writeErr := safe.WriterModify(ctx, runtime,
			hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, domain.Name),
			func(res *hypervisor.VirtualMachineDomainStatus) error {
				*res.TypedSpec() = status

				return nil
			},
		); writeErr != nil {
			errs = errors.Join(errs, fmt.Errorf("write domain status %q: %w", domain.Name, writeErr))
		}
	}

	return errors.Join(errs, safe.CleanupOutputs[*hypervisor.VirtualMachineDomainStatus](ctx, runtime))
}

func observeDomain(client libvirtdomain.Client, domain libvirtdomain.Domain) hypervisor.VirtualMachineDomainStatusSpec {
	status := hypervisor.VirtualMachineDomainStatusSpec{UUID: domain.UUID.String()}

	info, err := client.Info(domain)
	if err != nil {
		status.PowerState = hypervisor.VirtualMachinePowerStateUnknown
		status.Error = err.Error()

		return status
	}

	status.State = info.State
	status.MaxMemoryKiB = info.MaxMemoryKiB
	status.MemoryKiB = info.MemoryKiB
	status.VCPUs = info.VCPUs

	switch libvirt.DomainState(info.State) {
	case libvirt.DomainRunning, libvirt.DomainBlocked:
		status.PowerState = hypervisor.VirtualMachinePowerStateRunning
	case libvirt.DomainShutoff:
		status.PowerState = hypervisor.VirtualMachinePowerStateStopped
	case libvirt.DomainNostate, libvirt.DomainPaused, libvirt.DomainShutdown,
		libvirt.DomainCrashed, libvirt.DomainPmsuspended:
		status.PowerState = hypervisor.VirtualMachinePowerStateUnknown
	default:
		status.PowerState = hypervisor.VirtualMachinePowerStateUnknown
	}

	return status
}
