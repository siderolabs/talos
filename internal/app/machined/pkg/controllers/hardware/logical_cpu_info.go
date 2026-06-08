// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/prometheus/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// DefaultSysfsCPUPath is the sysfs root for per-logical-CPU directories.
const DefaultSysfsCPUPath = "/sys/devices/system/cpu"

// LogicalCPUInfoController populates per-logical-CPU information from procfs
// and sysfs, writing one LogicalCPUInfo per logical CPU the kernel reports.
// Reconciles on startup and on CPU hot-plug uevents.
type LogicalCPUInfoController struct {
	// ProcfsPath is the mount point of procfs. Defaults to /proc when empty.
	ProcfsPath string
	// SysfsCPUPath is the path to /sys/devices/system/cpu. Defaults to that
	// when empty.
	SysfsCPUPath string
}

// Name implements controller.Controller interface.
func (ctrl *LogicalCPUInfoController) Name() string {
	return "hardware.LogicalCPUInfoController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LogicalCPUInfoController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *LogicalCPUInfoController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.LogicalCPUInfoType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *LogicalCPUInfoController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	procfsPath := ctrl.ProcfsPath
	if procfsPath == "" {
		procfsPath = procfs.DefaultMountPoint
	}

	sysfsCPUPath := ctrl.SysfsCPUPath
	if sysfsCPUPath == "" {
		sysfsCPUPath = DefaultSysfsCPUPath
	}

	fs, err := procfs.NewFS(procfsPath)
	if err != nil {
		return fmt.Errorf("error opening procfs %q: %w", procfsPath, err)
	}

	// nil channel never fires in select; covers container mode where the
	// netlink socket may be unavailable.
	var hotplugCh <-chan *kobject.Event

	if watcher, werr := kobject.NewWatcher(logger); werr != nil {
		logger.Info("kobject watcher unavailable, falling back to one-shot reconcile", zap.Error(werr))
	} else {
		defer watcher.Close() //nolint:errcheck

		hotplugCh = watcher.Run("cpu")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case ev, ok := <-hotplugCh:
			if !ok {
				// Watcher closed the channel (errored out or shut down).
				// Disable this case so the closed channel does not spin
				// the select, and skip the reconcile since no CPU event
				// actually occurred.
				logger.Info("kobject watcher channel closed; hot-plug events will not be tracked")

				hotplugCh = nil

				continue
			}

			if ev == nil {
				continue
			}

			logger.Debug("CPU hot-plug event, reconciling",
				zap.String("action", string(ev.Action)),
				zap.String("path", ev.DevicePath),
			)
		}

		infos, err := fs.CPUInfo()
		if err != nil {
			return fmt.Errorf("error reading cpuinfo: %w", err)
		}

		r.StartTrackingOutputs()

		for _, info := range infos {
			id := "cpu" + strconv.FormatUint(uint64(info.Processor), 10)

			socket := readTopologyID(sysfsCPUPath, id, "physical_package_id")
			core := readTopologyID(sysfsCPUPath, id, "core_id")
			numa := readNUMANode(sysfsCPUPath, id)

			if err := safe.WriterModify(ctx, r, hardware.NewLogicalCPUInfo(id), func(res *hardware.LogicalCPUInfo) error {
				spec := res.TypedSpec()
				spec.Microcode = info.Microcode
				spec.Socket = socket
				spec.Core = core
				spec.NumaNode = numa
				spec.Bugs = info.Bugs

				return nil
			}); err != nil {
				return fmt.Errorf("error updating LogicalCPUInfo resource %q: %w", id, err)
			}
		}

		if err := r.CleanupOutputs(
			ctx,
			resource.NewMetadata(hardware.NamespaceName, hardware.LogicalCPUInfoType, "", resource.VersionUndefined),
		); err != nil {
			return fmt.Errorf("error cleaning up LogicalCPUInfo outputs: %w", err)
		}

		logger.Debug("populated logical CPU info", zap.Int("count", len(infos)))
	}
}

// readTopologyID reads an integer value from
// <sysfsCPUPath>/<cpuID>/topology/<file>. Returns 0 if the file is absent or
// the kernel reports a negative value (i.e. unknown).
func readTopologyID(sysfsCPUPath, cpuID, file string) uint32 {
	data, err := os.ReadFile(filepath.Join(sysfsCPUPath, cpuID, "topology", file))
	if err != nil {
		return 0
	}

	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}

	return uint32(n)
}

// readNUMANode resolves the per-CPU NUMA node from /sys/devices/system/cpu/<cpuID>.
// The kernel exposes the binding as a "node<N>" entry (a symlink to the NUMA
// node directory) inside the CPU directory; only the entry name is inspected,
// so the dirent type does not matter. Returns 0 if no such entry exists.
func readNUMANode(sysfsCPUPath, cpuID string) uint32 {
	entries, err := os.ReadDir(filepath.Join(sysfsCPUPath, cpuID))
	if err != nil {
		return 0
	}

	for _, e := range entries {
		name := e.Name()

		if !strings.HasPrefix(name, "node") {
			continue
		}

		n, err := strconv.ParseUint(name[len("node"):], 10, 32)
		if err != nil {
			continue
		}

		return uint32(n)
	}

	return 0
}
