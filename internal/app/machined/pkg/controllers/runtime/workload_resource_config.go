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
	"github.com/prometheus/procfs"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
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
// published only once the machine configuration is, so consumers can tell a pending projection (no
// resource) from the resolved policy. The host memory comes from the NUMA topology, which follows
// hotplug, or from MemTotal while the topology is missing. In container mode the projected policy is
// always empty.
type WorkloadResourceConfigController struct {
	V1Alpha1Mode machineruntime.Mode
	ProcfsPath   string
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
	if ctrl.ProcfsPath == "" {
		ctrl.ProcfsPath = procfs.DefaultMountPoint
	}

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

	var spec *runtime.WorkloadMemorySpecSpec

	if err == nil && cfg.Config() != nil {
		var (
			projected runtime.WorkloadMemorySpecSpec
			deferred  bool
		)

		if projected, err = ctrl.project(ctx, r, cfg.Config()); err != nil {
			return false, err
		}

		if deferred, err = deferCapacityChange(ctx, r, projected, settled); err != nil || deferred {
			return deferred, err
		}

		spec = &projected
	}

	r.StartTrackingOutputs()

	if spec != nil {
		if err = safe.WriterModify(ctx, r, runtime.NewWorkloadMemorySpec(), func(res *runtime.WorkloadMemorySpec) error {
			*res.TypedSpec() = *spec

			return nil
		}); err != nil {
			return false, fmt.Errorf("error updating workload memory spec: %w", err)
		}
	}

	return false, safe.CleanupOutputs[*runtime.WorkloadMemorySpec](ctx, r)
}

// deferCapacityChange holds back a host memory change until it settles.
func deferCapacityChange(ctx context.Context, r controller.Reader, spec runtime.WorkloadMemorySpecSpec, settled bool) (bool, error) {
	if settled {
		return false, nil
	}

	published, err := safe.ReaderGetByID[*runtime.WorkloadMemorySpec](ctx, r, runtime.WorkloadMemorySpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("error getting workload memory spec: %w", err)
	}

	return published.TypedSpec().MemoryCapacity != spec.MemoryCapacity, nil
}

// project resolves the limits in bytes.
func (ctrl *WorkloadResourceConfigController) project(ctx context.Context, r controller.Reader, cfg configcfg.Config) (runtime.WorkloadMemorySpecSpec, error) {
	if ctrl.V1Alpha1Mode.InContainer() {
		return runtime.WorkloadMemorySpecSpec{}, nil
	}

	capacity, err := ctrl.memoryCapacity(ctx, r)
	if err != nil {
		return runtime.WorkloadMemorySpecSpec{}, err
	}

	reserved := configcfg.ResolveMemoryReserved(cfg)

	var talosContainers, virtualMachines configcfg.MemoryLimit

	if resources := cfg.WorkloadResourceConfig(); resources != nil {
		talosContainers = resources.TalosContainersMemoryLimit()
		virtualMachines = resources.VirtualMachinesMemoryLimit()
	}

	return runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: talosRootMemoryLimit(talosContainers, capacity, reserved),
		VirtualMachinesLimit: talosRootMemoryLimit(virtualMachines, capacity, reserved),
		MemoryCapacity:       capacity,
		MemoryReserved:       reserved,
	}, nil
}

// memoryCapacity returns the host memory: NUMATopologyController drops the topology when it cannot read it, and
// the limits must not wait for it.
func (ctrl *WorkloadResourceConfigController) memoryCapacity(ctx context.Context, r controller.Reader) (uint64, error) {
	topology, err := safe.ReaderGetByID[*hardware.NUMATopology](ctx, r, hardware.NUMATopologyID)
	if err == nil {
		return topology.TypedSpec().MemoryTotalBytes(), nil
	}

	if !state.IsNotFoundError(err) {
		return 0, fmt.Errorf("error getting NUMA topology: %w", err)
	}

	fs, err := procfs.NewFS(ctrl.ProcfsPath)
	if err != nil {
		return 0, err
	}

	meminfo, err := fs.Meminfo()
	if err != nil {
		return 0, fmt.Errorf("error reading meminfo: %w", err)
	}

	if meminfo.MemTotalBytes == nil {
		return 0, fmt.Errorf("MemTotal is missing from %s/meminfo", ctrl.ProcfsPath)
	}

	return *meminfo.MemTotalBytes, nil
}

// talosRootMemoryLimit defaults an omitted Talos-owned root to all memory beyond the reservation.
//
// A host with no memory beyond the reservation keeps the omitted root unlimited instead of failing to apply a one-byte cap.
func talosRootMemoryLimit(limit configcfg.MemoryLimit, capacity, reserved uint64) uint64 {
	if limit.IsZero() {
		if capacity <= reserved {
			return 0
		}

		limit.Percent = 100
	}

	return limit.Resolve(capacity, reserved)
}
