// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/cadvisor/lib/utils/sysfs"
	"github.com/google/cadvisor/lib/utils/sysinfo"
	"github.com/prometheus/procfs"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// NUMATopologyController publishes Linux NUMA inventory and watches hotplug events.
type NUMATopologyController struct {
	V1Alpha1Mode runtimetalos.Mode
	SysfsPath    string
	ProcfsPath   string
	// ReconcileCh triggers an additional reconcile for testing.
	ReconcileCh <-chan struct{}
}

// Name implements controller.Controller.
func (ctrl *NUMATopologyController) Name() string {
	return "hardware.NUMATopologyController"
}

// Inputs implements controller.Controller.
func (ctrl *NUMATopologyController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller.
func (ctrl *NUMATopologyController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.NUMATopologyType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller.
func (ctrl *NUMATopologyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error { //nolint:gocyclo
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	if ctrl.SysfsPath == "" {
		ctrl.SysfsPath = "/sys"
	}

	if ctrl.ProcfsPath == "" {
		ctrl.ProcfsPath = procfs.DefaultMountPoint
	}

	watcher, err := kobject.NewWatcher(logger)
	if err != nil {
		return err
	}
	defer watcher.Close() //nolint:errcheck

	watchCh := watcher.Run("cpu", "node", "memory")
	rateLimitedTrigger := trigger.NewRateLimitedTrigger(ctx, r, 1, 1)
	rateLimitedTrigger.QueueReconcile()

	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watchCh:
			if !ok {
				select {
				case err := <-watcher.ErrCh():
					return err
				default:
					return nil
				}
			}

			rateLimitedTrigger.QueueReconcile()
		case err := <-watcher.ErrCh():
			return err
		case <-r.EventCh():
			if err := ctrl.reconcile(ctx, r); err != nil {
				return err
			}

			r.ResetRestartBackoff()
		case <-ctrl.ReconcileCh:
			if err := ctrl.reconcile(ctx, r); err != nil {
				return err
			}

			r.ResetRestartBackoff()
		}
	}
}

func (ctrl *NUMATopologyController) reconcile(ctx context.Context, r controller.Runtime) error {
	spec, err := ctrl.readTopology()
	if err != nil {
		// A failed or changing snapshot must not leave stale inventory available to consumers.
		if destroyErr := r.Destroy(ctx, hardware.NewNUMATopology().Metadata()); destroyErr != nil && !state.IsNotFoundError(destroyErr) {
			return errors.Join(err, destroyErr)
		}

		return err
	}

	return safe.WriterModify(ctx, r, hardware.NewNUMATopology(), func(res *hardware.NUMATopology) error {
		*res.TypedSpec() = *spec

		return nil
	})
}

func readCPUSet(path string) ([]uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	set, err := cpuset.Parse(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	result := make([]uint32, 0, set.Size())
	for _, id := range set.List() {
		if uint64(id) > math.MaxUint32 {
			return nil, fmt.Errorf("ID %d in %s exceeds uint32", id, path)
		}

		result = append(result, uint32(id))
	}

	return result, nil
}

func (ctrl *NUMATopologyController) readTopology() (*hardware.NUMATopologySpec, error) { //nolint:gocyclo
	cpuPath := filepath.Join(ctrl.SysfsPath, "devices/system/cpu")
	nodePath := filepath.Join(ctrl.SysfsPath, "devices/system/node")

	present, err := readCPUSet(filepath.Join(cpuPath, "present"))
	if err != nil {
		return nil, err
	}

	online, err := readCPUSet(filepath.Join(cpuPath, "online"))
	if err != nil {
		return nil, err
	}

	for _, id := range online {
		if !slices.Contains(present, id) {
			return nil, fmt.Errorf("online CPU %d is not present", id)
		}
	}

	spec := &hardware.NUMATopologySpec{PresentCPUs: present, OnlineCPUs: online}
	_, err = os.Stat(nodePath)

	fallback := os.IsNotExist(err)
	if err != nil && !fallback {
		return nil, err
	}

	var nodes []uint32

	if fallback {
		fs, fsErr := procfs.NewFS(ctrl.ProcfsPath)
		if fsErr != nil {
			return nil, fsErr
		}

		mem, memErr := fs.Meminfo()
		if memErr != nil {
			return nil, memErr
		}

		if mem.MemTotal == nil || *mem.MemTotal > math.MaxUint64/1024 {
			return nil, errors.New("invalid MemTotal in /proc/meminfo")
		}

		spec.Nodes = []hardware.NUMANodeSpec{{ID: 0, CPUs: slices.Clone(present), MemoryTotalBytes: *mem.MemTotal * 1024}}
	} else {
		nodes, err = readCPUSet(filepath.Join(nodePath, "online"))
		if err != nil {
			return nil, err
		}

		if len(nodes) == 0 {
			return nil, errors.New("no online NUMA nodes")
		}

		spec.Nodes, err = readNUMANodes(nodePath, nodes, present, online)
		if err != nil {
			return nil, err
		}
	}

	for _, cpu := range online {
		found := false
		for _, node := range spec.Nodes {
			found = found || slices.Contains(node.CPUs, cpu)
		}

		if !found {
			return nil, fmt.Errorf("online CPU %d has no NUMA node", cpu)
		}
	}

	if err := ctrl.validateMembership(spec, nodes, fallback); err != nil {
		return nil, err
	}

	return spec, nil
}

// Recheck membership after collecting the nodes. Sysfs is not an atomic snapshot.
func (ctrl *NUMATopologyController) validateMembership(spec *hardware.NUMATopologySpec, nodes []uint32, fallback bool) error { //nolint:gocyclo
	cpuPath := filepath.Join(ctrl.SysfsPath, "devices/system/cpu")
	nodePath := filepath.Join(ctrl.SysfsPath, "devices/system/node")

	afterPresent, err := readCPUSet(filepath.Join(cpuPath, "present"))
	if err != nil {
		return err
	}

	afterOnline, err := readCPUSet(filepath.Join(cpuPath, "online"))
	if err != nil {
		return err
	}

	if !slices.Equal(spec.PresentCPUs, afterPresent) || !slices.Equal(spec.OnlineCPUs, afterOnline) {
		return errors.New("CPU membership changed while reading NUMA topology")
	}

	if fallback {
		if _, err := os.Stat(nodePath); !os.IsNotExist(err) {
			return errors.New("NUMA node interface changed while reading topology")
		}

		return nil
	}

	afterNodes, err := readCPUSet(filepath.Join(nodePath, "online"))
	if err != nil {
		return err
	}

	if !slices.Equal(nodes, afterNodes) {
		return errors.New("node membership changed while reading NUMA topology")
	}

	for _, node := range spec.Nodes {
		cpus, err := readCPUSet(filepath.Join(nodePath, fmt.Sprintf("node%d/cpulist", node.ID)))
		if err != nil {
			return err
		}

		if !slices.Equal(node.CPUs, cpus) {
			return errors.New("node CPU membership changed while reading NUMA topology")
		}
	}

	return nil
}

func readNUMANodes(nodePath string, ids, present, online []uint32) ([]hardware.NUMANodeSpec, error) { //nolint:gocyclo
	fs := &numaSysfs{SysFs: sysfs.NewRealSysFs(), online: online}
	for _, id := range ids {
		fs.nodes = append(fs.nodes, filepath.Join(nodePath, fmt.Sprintf("node%d", id)))
	}

	discovered, _, err := sysinfo.GetNodesInfo(fs)
	if err != nil || fs.err != nil {
		return nil, errors.Join(err, fs.err)
	}

	nodes := make([]hardware.NUMANodeSpec, 0, len(ids))
	assigned := make(map[uint32]struct{})

	for _, node := range discovered {
		id := uint32(node.Id)
		path := filepath.Join(nodePath, fmt.Sprintf("node%d", id))

		cpus, err := readCPUSet(filepath.Join(path, "cpulist"))
		if err != nil {
			return nil, err
		}

		for _, cpu := range cpus {
			if !slices.Contains(present, cpu) {
				return nil, fmt.Errorf("node %d CPU %d is not present", id, cpu)
			}

			if _, exists := assigned[cpu]; exists {
				return nil, fmt.Errorf("CPU %d belongs to multiple nodes", cpu)
			}

			assigned[cpu] = struct{}{}
		}

		discoveredCPUs := make([]uint32, 0, len(cpus))

		for _, core := range node.Cores {
			for _, cpu := range core.Threads {
				discoveredCPUs = append(discoveredCPUs, uint32(cpu))
			}
		}

		// cAdvisor reports online threads; retain the offline membership too.
		for _, cpu := range cpus {
			if !slices.Contains(online, cpu) {
				discoveredCPUs = append(discoveredCPUs, cpu)
			}
		}

		slices.Sort(discoveredCPUs)

		if !slices.Equal(cpus, discoveredCPUs) {
			return nil, fmt.Errorf("node %d CPU topology is incomplete or changed during discovery", id)
		}

		nodes = append(nodes, hardware.NUMANodeSpec{ID: id, CPUs: discoveredCPUs, MemoryTotalBytes: node.Memory})
	}

	return nodes, nil
}

// cAdvisor tolerates missing memory information and filters CPUs against the live
// host. Preserve read errors and use the membership snapshot for this collection.
type numaSysfs struct {
	sysfs.SysFs
	err    error
	nodes  []string
	online []uint32
}

func (fs *numaSysfs) GetNodesPaths() ([]string, error) { return fs.nodes, nil }

func (fs *numaSysfs) GetMemInfo(nodeDir string) (string, error) {
	info, err := fs.SysFs.GetMemInfo(nodeDir)
	fs.err = errors.Join(fs.err, err)

	return info, err
}

func (fs *numaSysfs) IsCPUOnline(path string) bool {
	id, err := strconv.ParseUint(strings.TrimPrefix(filepath.Base(path), "cpu"), 10, 32)
	if err != nil {
		fs.err = errors.Join(fs.err, err)

		return false
	}

	return slices.Contains(fs.online, uint32(id))
}
