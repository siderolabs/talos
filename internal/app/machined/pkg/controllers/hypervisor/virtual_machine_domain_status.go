// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	// Open is the long-lived observation session that feeds Watch and lifecycle queries.
	Open func(context.Context) (libvirtdomain.Client, error)
	// OpenBounded is used for the guest-agent query, which the controller does not pace: a hung
	// agent on the inventory session would stall every VM observed after it in the same pass.
	OpenBounded func(context.Context) (libvirtdomain.Client, error)
	Watch       func(context.Context) (<-chan struct{}, error)
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

// agentPollInterval drives re-queries of qemu-guest-agent between libvirt lifecycle events, which
// do not announce agent connect.
const agentPollInterval = 15 * time.Second

// Run implements controller.Controller.
func (ctrl *VirtualMachineDomainStatusController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	var session domainObservationSession
	defer session.close()

	poll := time.NewTicker(agentPollInterval)
	defer poll.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
			if err := ctrl.handleServiceEvent(ctx, runtime, &session); err != nil {
				return err
			}
		case _, ok := <-session.events:
			if err := ctrl.dispatchWatchEvent(ctx, runtime, &session, ok); err != nil {
				return err
			}
		case <-poll.C:
			if err := ctrl.handlePoll(ctx, runtime, &session); err != nil {
				return err
			}
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDomainStatusController) dispatchWatchEvent(
	ctx context.Context,
	runtime controller.Runtime,
	session *domainObservationSession,
	ok bool,
) error {
	err := ctrl.handleWatchEvent(ctx, runtime, session, ok)
	if err == nil {
		return nil
	}

	if ctx.Err() != nil {
		return nil //nolint:nilerr // cancellation is not an observation failure.
	}

	return err
}

func (ctrl *VirtualMachineDomainStatusController) handlePoll(
	ctx context.Context,
	runtime controller.Runtime,
	session *domainObservationSession,
) error {
	if session.client == nil {
		return nil
	}

	if err := ctrl.reconcile(ctx, runtime, session.client); err != nil {
		session.close()

		return ctrl.observationError(ctx, runtime, err)
	}

	return nil
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
			// Dropped alongside the error: the last values were only true for the previous
			// observation, and keeping them would surface stale IPs on an unreachable VM.
			resource.TypedSpec().Interfaces = nil

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
		status := ctrl.observeDomain(ctx, client, domain)

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

func (ctrl *VirtualMachineDomainStatusController) observeDomain(ctx context.Context, client libvirtdomain.Client, domain libvirtdomain.Domain) hypervisor.VirtualMachineDomainStatusSpec {
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

	// Agent query runs on a short-lived sub-session so a hung agent sheds on its own deadline
	// instead of stalling the inventory session for every later VM.
	if status.PowerState == hypervisor.VirtualMachinePowerStateRunning && ctrl.OpenBounded != nil {
		status.Interfaces = ctrl.queryGuestInterfaces(ctx, domain)
	}

	return status
}

// queryGuestInterfaces opens a short-lived session and returns what the guest agent reports.
// Any failure yields nil; the status's own error field surfaces unreachability.
func (ctrl *VirtualMachineDomainStatusController) queryGuestInterfaces(ctx context.Context, domain libvirtdomain.Domain) []hypervisor.VirtualMachineGuestInterfaceSpec {
	client, err := ctrl.OpenBounded(ctx)
	if err != nil {
		return nil
	}

	defer client.Close()

	ifaces, err := client.GuestInterfaces(domain)
	if err != nil {
		return nil
	}

	return convertGuestInterfaces(ifaces)
}

func convertGuestInterfaces(src []libvirtdomain.GuestInterface) []hypervisor.VirtualMachineGuestInterfaceSpec {
	out := make([]hypervisor.VirtualMachineGuestInterfaceSpec, 0, len(src))

	for _, iface := range src {
		converted := hypervisor.VirtualMachineGuestInterfaceSpec{
			Name:         iface.Name,
			HardwareAddr: iface.HardwareAddr,
		}

		for _, addr := range iface.IPs {
			if addr.Addr().IsLoopback() {
				continue
			}

			converted.IPAddresses = append(converted.IPAddresses, addr)
		}

		out = append(out, converted)
	}

	return out
}
