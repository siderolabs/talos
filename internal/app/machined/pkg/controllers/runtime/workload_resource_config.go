// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// CapacitySettleDelay holds back a host memory change until no other one followed for this long.
//
// Memory hotplug onlines or offlines one memory block per uevent, and the kubelet resolves a kubepods limit against
// the published capacity, restarting on every change, so a burst must be published once.
const CapacitySettleDelay = 5 * time.Second

// WorkloadResourceConfigController resolves WorkloadResourceConfig into WorkloadMemorySpec.
//
// The roots the document leaves out, or all of them without the document, are capped at the host memory
// minus the reservation, which defaults to the kubelet's default systemReserved.memory. The spec is
// published only once the machine configuration and the host memory inventory are,
// so consumers can tell a pending projection (no resource) from the resolved policy. A published spec
// is kept while the inventory is briefly gone during hotplug. In container mode the projected policy
// is always empty.
type WorkloadResourceConfigController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Name() string {
	return "runtime.WorkloadResourceConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.NUMATopologyType,
			ID:        optional.Some(hardware.NUMATopologyID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.WorkloadMemorySpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	settle := time.NewTimer(CapacitySettleDelay)
	settle.Stop()

	defer settle.Stop()

	for {
		settled := false

		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-settle.C:
			settled = true
		}

		deferred, err := ctrl.reconcile(ctx, r, settled)
		if err != nil {
			return err
		}

		if deferred {
			settle.Reset(CapacitySettleDelay)
		}

		r.ResetRestartBackoff()
	}
}

// reconcile publishes the resolved spec; deferred reports a host memory change held back until it settles.
func (ctrl *WorkloadResourceConfigController) reconcile(ctx context.Context, r controller.Runtime, settled bool) (bool, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting machine config: %w", err)
	}

	var (
		spec  runtime.WorkloadMemorySpecSpec
		ready bool
	)

	if err == nil && cfg.Config() != nil {
		spec, ready, err = ctrl.project(ctx, r, cfg.Config())
		if err != nil {
			return false, err
		}

		var hold, deferred bool

		if hold, deferred, err = holdPublished(ctx, r, spec, ready, settled); err != nil || hold {
			return deferred, err
		}
	}

	r.StartTrackingOutputs()

	if ready {
		if err = safe.WriterModify(ctx, r, runtime.NewWorkloadMemorySpec(), func(res *runtime.WorkloadMemorySpec) error {
			*res.TypedSpec() = spec

			return nil
		}); err != nil {
			return false, fmt.Errorf("error updating workload memory spec: %w", err)
		}
	}

	return false, safe.CleanupOutputs[*runtime.WorkloadMemorySpec](ctx, r)
}

// holdPublished keeps a published spec while the inventory is briefly gone during hotplug, and while a host memory
// change has not settled yet; deferred asks for a retry once it does.
func holdPublished(ctx context.Context, r controller.Reader, spec runtime.WorkloadMemorySpecSpec, ready, settled bool) (hold, deferred bool, err error) {
	published, err := safe.ReaderGetByID[*runtime.WorkloadMemorySpec](ctx, r, runtime.WorkloadMemorySpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, false, nil
		}

		return false, false, fmt.Errorf("error getting workload memory spec: %w", err)
	}

	if !ready {
		return true, false, nil
	}

	if !settled && published.TypedSpec().MemoryCapacity != spec.MemoryCapacity {
		return true, true, nil
	}

	return false, false, nil
}

// project resolves the limits in bytes; ready is false while the host memory inventory is missing.
func (ctrl *WorkloadResourceConfigController) project(ctx context.Context, r controller.Reader, cfg configcfg.Config) (runtime.WorkloadMemorySpecSpec, bool, error) {
	if ctrl.V1Alpha1Mode.InContainer() {
		return runtime.WorkloadMemorySpecSpec{}, true, nil
	}

	topology, err := safe.ReaderGetByID[*hardware.NUMATopology](ctx, r, hardware.NUMATopologyID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return runtime.WorkloadMemorySpecSpec{}, false, nil
		}

		return runtime.WorkloadMemorySpecSpec{}, false, fmt.Errorf("error getting NUMA topology: %w", err)
	}

	var machineType machine.Type

	if cfg.Machine() != nil {
		machineType = cfg.Machine().Type()
	}

	reserved := configcfg.DefaultMemoryReserved(machineType)

	var talosContainers, virtualMachines uint64

	if resources := cfg.WorkloadResourceConfig(); resources != nil {
		reserved = resources.MemoryReserved().ValueOr(reserved)
		talosContainers = resources.TalosContainersMemoryPercent()
		virtualMachines = resources.VirtualMachinesMemoryPercent()
	}

	capacity := topology.TypedSpec().MemoryTotalBytes()

	return runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: talosRootMemoryLimit(talosContainers, capacity, reserved),
		VirtualMachinesLimit: talosRootMemoryLimit(virtualMachines, capacity, reserved),
		MemoryCapacity:       capacity,
		MemoryReserved:       reserved,
	}, true, nil
}

// talosRootMemoryLimit defaults an omitted Talos-owned root to all memory beyond the reservation.
//
// A host with no memory beyond the reservation keeps the omitted root unlimited instead of failing to apply a one-byte cap.
func talosRootMemoryLimit(percent, capacity, reserved uint64) uint64 {
	if percent == 0 {
		if capacity <= reserved {
			return 0
		}

		percent = 100
	}

	return configcfg.MemoryLimit(percent, capacity, reserved)
}
