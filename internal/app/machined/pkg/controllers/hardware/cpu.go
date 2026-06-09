// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/prometheus/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// CPUInfoController populates the Linux kernel view of the CPUs (parsed from /proc/cpuinfo) as CPUCore resources.
//
// CPUs are grouped by physical core, producing a single resource per core. The controller watches the `cpu`
// kobject subsystem, so CPU hotplug events update the output.
type CPUInfoController struct {
	V1Alpha1Mode runtimetalos.Mode

	// ProcfsPath is the procfs mount point, defaults to /proc. Overridable for testing.
	ProcfsPath string

	// ReconcileCh triggers an additional reconcile on each receive. Intended for testing only.
	ReconcileCh <-chan struct{}
}

// Name implements controller.Controller interface.
func (ctrl *CPUInfoController) Name() string {
	return "hardware.CPUInfoController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CPUInfoController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *CPUInfoController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.CPUCoreType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CPUInfoController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		// in container, reading this information will return host CPU info, not container-limited info,
		// skip populating any resources
		return nil
	}

	if ctrl.ProcfsPath == "" {
		ctrl.ProcfsPath = procfs.DefaultMountPoint
	}

	// CPU hotplug events arrive once per logical CPU, so a single hotplug of a socket produces a burst.
	// Use a rate-limited trigger to coalesce the burst into a single reconcile.
	rateLimitedTrigger := *trigger.NewRateLimitedTrigger(ctx, r, 1, 1)

	var (
		watchCh <-chan *kobject.Event
		errCh   <-chan error
	)

	watcher, err := kobject.NewWatcher(logger)
	if err != nil {
		return fmt.Errorf("failed to create kobject watcher: %w", err)
	}

	watchCh = watcher.Run("cpu")
	errCh = watcher.ErrCh()

	defer watcher.Close() //nolint:errcheck

	return ctrl.runWatchLoop(ctx, r, rateLimitedTrigger, watchCh, errCh)
}

func (ctrl *CPUInfoController) runWatchLoop( //nolint:gocyclo
	ctx context.Context, r controller.Runtime, rateLimitedTrigger trigger.RateLimitedTrigger, watchCh <-chan *kobject.Event, errCh <-chan error,
) error {
	// Initial reconcile to expose resources immediately.
	rateLimitedTrigger.QueueReconcile()

	for {
		select {
		case _, ok := <-watchCh:
			if !ok {
				select {
				case err := <-errCh:
					return fmt.Errorf("error watching for CPU changes: %w", err)
				default:
					return nil
				}
			}

			rateLimitedTrigger.QueueReconcile()
		case err := <-errCh:
			return fmt.Errorf("error watching for CPU changes: %w", err)
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcile(ctx, r); err != nil {
				return fmt.Errorf("error reconciling CPUCore resources: %w", err)
			}
		// For testing only.
		case <-ctrl.ReconcileCh:
			if err := ctrl.reconcile(ctx, r); err != nil {
				return fmt.Errorf("error reconciling CPUCore resources: %w", err)
			}
		}
	}
}

func (ctrl *CPUInfoController) reconcile(ctx context.Context, r controller.Runtime) error {
	fs, err := procfs.NewFS(ctrl.ProcfsPath)
	if err != nil {
		return fmt.Errorf("error opening procfs at %q: %w", ctrl.ProcfsPath, err)
	}

	cpus, err := fs.CPUInfo()
	if err != nil {
		return fmt.Errorf("error reading cpuinfo: %w", err)
	}

	cores := GroupCPUInfo(cpus)

	r.StartTrackingOutputs()

	for id, spec := range cores {
		if err := safe.WriterModify(ctx, r, hardware.NewCPUCore(id), func(res *hardware.CPUCore) error {
			*res.TypedSpec() = spec

			return nil
		}); err != nil {
			return fmt.Errorf("error updating CPUCore resource %q: %w", id, err)
		}
	}

	return r.CleanupOutputs(
		ctx,
		resource.NewMetadata(hardware.NamespaceName, hardware.CPUCoreType, "", resource.VersionUndefined),
	)
}

// GroupCPUInfo groups /proc/cpuinfo entries by physical core, producing a single CPUCoreSpec per core.
//
// Cores are keyed by `<physical id>-<core id>`. On architectures which don't report physical/core ids
// (e.g. some ARM systems), each logical CPU is treated as its own core, keyed by the processor number.
func GroupCPUInfo(cpus []procfs.CPUInfo) map[string]hardware.CPUCoreSpec {
	cores := map[string]hardware.CPUCoreSpec{}

	for _, cpu := range cpus {
		id := coreID(cpu)

		spec, ok := cores[id]
		if !ok {
			spec = hardware.CPUCoreSpec{
				Socket:           cpu.PhysicalID,
				CoreID:           cpu.CoreID,
				VendorID:         cpu.VendorID,
				CPUFamily:        cpu.CPUFamily,
				Model:            cpu.Model,
				ModelName:        cpu.ModelName,
				Stepping:         cpu.Stepping,
				Microcode:        cpu.Microcode,
				CacheSize:        cpu.CacheSize,
				CoresPerSocket:   uint32(cpu.CPUCores),
				ThreadsPerSocket: uint32(cpu.Siblings),
				Flags:            cpu.Flags,
				Bugs:             cpu.Bugs,
				BogoMips:         cpu.BogoMips,
				AddressSizes:     cpu.AddressSizes,
			}
		}

		spec.LogicalCPUs = append(spec.LogicalCPUs, uint32(cpu.Processor))

		cores[id] = spec
	}

	for id, spec := range cores {
		slices.Sort(spec.LogicalCPUs)
		cores[id] = spec
	}

	return cores
}

// coreID computes a stable identifier for the core a logical CPU belongs to.
func coreID(cpu procfs.CPUInfo) string {
	if cpu.PhysicalID != "" && cpu.CoreID != "" {
		return cpu.PhysicalID + "-" + cpu.CoreID
	}

	return strconv.FormatUint(uint64(cpu.Processor), 10)
}
