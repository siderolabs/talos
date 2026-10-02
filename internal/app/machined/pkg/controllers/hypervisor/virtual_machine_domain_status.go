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
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const virtqemudNotReadyError = "virtqemud service is not ready"

// VirtualMachineDomainStatusController inventories all libvirt domains, regardless of Talos ownership.
type VirtualMachineDomainStatusController struct {
	V1Alpha1Mode machineruntime.Mode
	Open         func(context.Context) (libvirtdomain.Client, error)
	Watch        func(context.Context) (<-chan struct{}, error)
}

type domainObservationSession struct {
	client      libvirtdomain.Client
	cancelWatch context.CancelFunc
	events      <-chan struct{}
}

func (session *domainObservationSession) close() {
	if session.cancelWatch != nil {
		session.cancelWatch()
		session.cancelWatch = nil
	}

	if session.client != nil {
		session.client.Close()
		session.client = nil
	}

	session.events = nil
}

// Name implements controller.Controller.
func (*VirtualMachineDomainStatusController) Name() string {
	return "hypervisor.VirtualMachineDomainStatusController"
}

// Inputs implements controller.Controller.
func (*VirtualMachineDomainStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(virtqemudServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller.
func (*VirtualMachineDomainStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineDomainStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller.
func (ctrl *VirtualMachineDomainStatusController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	var session domainObservationSession
	defer session.close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
			if err := ctrl.handleServiceEvent(ctx, runtime, &session); err != nil {
				return err
			}
		case _, ok := <-session.events:
			if err := ctrl.handleWatchEvent(ctx, runtime, &session, ok); err != nil {
				if ctx.Err() != nil {
					return nil
				}

				return err
			}
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDomainStatusController) handleServiceEvent(
	ctx context.Context,
	runtime controller.Runtime,
	session *domainObservationSession,
) error {
	ready, err := virtqemudReady(ctx, runtime)
	if err != nil {
		return err
	}

	if !ready {
		session.close()

		return ctrl.markUnavailable(ctx, runtime, virtqemudNotReadyError)
	}

	if session.client != nil {
		return nil
	}

	return ctrl.openSession(ctx, runtime, session)
}

func (ctrl *VirtualMachineDomainStatusController) openSession(
	ctx context.Context,
	runtime controller.Runtime,
	session *domainObservationSession,
) error {
	watchCtx, cancel := context.WithCancel(ctx)

	events, err := ctrl.Watch(watchCtx)
	if err != nil {
		cancel()

		return ctrl.observationError(ctx, runtime, fmt.Errorf("watch libvirt domains: %w", err))
	}

	session.cancelWatch = cancel
	session.events = events

	session.client, err = ctrl.Open(ctx)
	if err != nil {
		session.close()

		return ctrl.observationError(ctx, runtime, fmt.Errorf("open libvirt: %w", err))
	}

	// The watch is registered before the first inventory. Events arriving during
	// the inventory remain queued and cause a follow-up scan.
	if err = ctrl.reconcile(ctx, runtime, session.client); err != nil {
		session.close()

		return ctrl.observationError(ctx, runtime, err)
	}

	return nil
}

func (ctrl *VirtualMachineDomainStatusController) handleWatchEvent(
	ctx context.Context,
	runtime controller.Runtime,
	session *domainObservationSession,
	watchOpen bool,
) error {
	if watchOpen {
		if err := ctrl.reconcile(ctx, runtime, session.client); err != nil {
			session.close()

			return ctrl.observationError(ctx, runtime, err)
		}

		return nil
	}

	session.close()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	ready, err := virtqemudReady(ctx, runtime)
	if err != nil {
		return err
	}

	if !ready {
		return ctrl.markUnavailable(ctx, runtime, virtqemudNotReadyError)
	}

	return ctrl.observationError(ctx, runtime, errors.New("libvirt domain watch closed"))
}

func (ctrl *VirtualMachineDomainStatusController) observationError(
	ctx context.Context,
	r controller.ReaderWriter,
	err error,
) error {
	markErr := ctrl.markUnavailable(ctx, r, "libvirt observation unavailable: "+err.Error())

	return errors.Join(err, markErr)
}

func (ctrl *VirtualMachineDomainStatusController) markUnavailable(
	ctx context.Context,
	r controller.ReaderWriter,
	reason string,
) error {
	statuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("list virtual machine domain statuses: %w", err)
	}

	var errs error

	for status := range statuses.All() {
		if writeErr := safe.WriterModify(ctx, r, status, func(resource *hypervisor.VirtualMachineDomainStatus) error {
			resource.TypedSpec().PowerState = hypervisor.VirtualMachinePowerStateUnknown
			resource.TypedSpec().Error = reason

			return nil
		}); writeErr != nil {
			errs = errors.Join(errs, fmt.Errorf("mark domain status %q unavailable: %w", status.Metadata().ID(), writeErr))
		}
	}

	return errs
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
