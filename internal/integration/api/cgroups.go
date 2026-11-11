// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	k8sres "github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// CGroupsSuite ...
type CGroupsSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *CGroupsSuite) SuiteName() string {
	return "api.CGroupsSuite"
}

// SetupTest ...
func (suite *CGroupsSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *CGroupsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestCGroupsVersion tests that cgroups mount match expected version.
func (suite *CGroupsSuite) TestCGroupsVersion() {
	names := suite.listRootCgroup()

	suite.T().Log("detected cgroups v2")

	for _, subpath := range []string{
		"cgroup.controllers",
		"cgroup.max.depth",
		"cgroup.max.descendants",
		"cgroup.procs",
		"cgroup.stat",
		"cgroup.subtree_control",
		"cgroup.threads",
		"cpu.stat",
		"cpuset.cpus.effective",
		"cpuset.mems.effective",
		"init",
		"io.stat",
		"memory.numa_stat",
		"memory.stat",
		"podruntime",
		"system",
		constants.CgroupTalosContainersRoot,
		constants.CgroupVirtualMachines,
	} {
		suite.Assert().Contains(names, subpath)
	}
}

// TestCGroupsKubepods tests that kubelet creates the root cgroup for pods.
func (suite *CGroupsSuite) TestCGroupsKubepods() {
	if !suite.Capabilities().SupportsKubernetes {
		suite.T().Skip("cluster doesn't run Kubernetes, so kubelet never creates the kubepods cgroup")
	}

	suite.Assert().Contains(suite.listRootCgroup(), constants.CgroupKubepods)
}

// listRootCgroup returns the names of the entries in the root cgroup of a random node.
func (suite *CGroupsSuite) listRootCgroup() map[string]struct{} {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	stream, err := suite.Client.MachineService().List(ctx, &machineapi.ListRequest{Root: constants.CgroupMountPath})
	suite.Require().NoError(err)

	names := map[string]struct{}{}

	for {
		var info *machineapi.FileInfo

		info, err = stream.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				break
			}

			suite.Require().NoError(err)
		}

		names[filepath.Base(info.Name)] = struct{}{}
	}

	return names
}

// TestWorkloadMemoryReserve verifies the reservation the node resolves, from WorkloadResourceConfig or the
// kubelet default for the machine type, on the Talos-owned workload roots and the kubelet.
func (suite *CGroupsSuite) TestWorkloadMemoryReserve() {
	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("cgroups are nested in container mode, so the workload roots are not managed")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	cfg, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	machineType, err := safe.StateGetByID[*configres.MachineType](nodeCtx, suite.Client.State(), configres.MachineTypeID)
	suite.Require().NoError(err)

	reserved := configconfig.DefaultMemoryReserved(machineType.MachineType())
	percents := map[string]uint64{constants.CgroupTalosContainersRoot: 0, constants.CgroupVirtualMachines: 0}

	var kubepodsPercent uint64

	if doc := cfg.WorkloadResourceConfig(); doc != nil {
		reserved = doc.MemoryReserved().ValueOr(reserved)
		kubepodsPercent = doc.KubepodsMemoryPercent()
		percents[constants.CgroupTalosContainersRoot] = doc.TalosContainersMemoryPercent()
		percents[constants.CgroupVirtualMachines] = doc.VirtualMachinesMemoryPercent()
	}

	spec, err := safe.StateGetByID[*runtime.WorkloadMemorySpec](nodeCtx, suite.Client.State(), runtime.WorkloadMemorySpecID)
	suite.Require().NoError(err)
	suite.Require().Equal(reserved, spec.TypedSpec().MemoryReserved)

	// A root without a limit is capped at all of the memory beyond the reservation.
	for root, percent := range percents {
		if percent == 0 {
			percent = 100
		}

		suite.AssertWorkloadRootLimit(nodeCtx, root, percent)
	}

	kubeletSpec, err := safe.StateGetByID[*k8sres.KubeletSpec](nodeCtx, suite.Client.State(), k8sres.KubeletID)
	if state.IsNotFoundError(err) {
		return
	}

	suite.Require().NoError(err)

	// A kubepods limit derives systemReserved.memory from the host memory instead.
	if kubepodsPercent == 0 {
		systemReserved, _ := kubeletSpec.TypedSpec().Config["systemReserved"].(map[string]any)
		suite.Assert().Equal(strconv.FormatUint(reserved, 10), systemReserved["memory"], "kubelet systemReserved.memory")
	}
}

// workloadPidsRoots are the roots whose pids.max WorkloadPidsController enforces, with their reservations.
var workloadPidsRoots = []struct {
	cgroup   string
	reserved uint64
}{
	{constants.CgroupTalosContainersRoot, constants.CgroupTalosContainersReservedPids},
	{constants.CgroupVirtualMachines, constants.CgroupVirtualMachinesReservedPids},
}

// TestWorkloadPidsLimit verifies the workload roots' pids.max follows the host PID capacity, as kubepods does.
func (suite *CGroupsSuite) TestWorkloadPidsLimit() {
	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("cgroups are nested in container mode, so the workload roots are not managed")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	pidMax := suite.readSysctlUint(nodeCtx, "pid_max")
	threadsMax := suite.readSysctlUint(nodeCtx, "threads-max")

	suite.assertWorkloadPidsMax(nodeCtx, pidMax, threadsMax)

	if testing.Short() {
		return
	}

	original, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if slices.ContainsFunc(original.Documents(), func(doc configconfig.Document) bool {
		return doc.Kind() == runtimecfg.SysctlConfigKind
	}) {
		suite.T().Skipf("node %s already carries a %s document", node, runtimecfg.SysctlConfigKind)
	}

	originalBytes, err := original.Bytes()
	suite.Require().NoError(err)

	// lower than the Talos default, and still well above anything the node runs
	lowered := pidMax / 2

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		cleanupCtx = client.WithNode(cleanupCtx, node)

		_, applyErr := suite.Client.ApplyConfiguration(cleanupCtx, &machineapi.ApplyConfigurationRequest{
			Data: originalBytes,
			Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
		})
		if !suite.Assert().NoError(applyErr, "restore original configuration on node %s", node) {
			return
		}

		suite.assertWorkloadPidsMax(cleanupCtx, pidMax, threadsMax)
	})

	doc := runtimecfg.NewSysctlConfigV1Alpha1()
	doc.Params = map[string]string{"kernel.pid_max": strconv.FormatUint(lowered, 10)}

	suite.PatchMachineConfig(nodeCtx, doc)

	suite.assertWorkloadPidsMax(nodeCtx, lowered, threadsMax)
}

func (suite *CGroupsSuite) assertWorkloadPidsMax(nodeCtx context.Context, pidMax, threadsMax uint64) {
	suite.T().Helper()

	for _, root := range workloadPidsRoots {
		suite.AssertCgroupFile(nodeCtx, root.cgroup, "pids.max", expectedPidsMax(pidMax, threadsMax, root.reserved))
	}
}

func (suite *CGroupsSuite) readSysctlUint(nodeCtx context.Context, name string) uint64 {
	value, err := strconv.ParseUint(suite.ReadFile(nodeCtx, filepath.Join(constants.ProcSysPath, "kernel", name)), 10, 64)
	suite.Require().NoError(err)

	return value
}

// expectedPidsMax mirrors kubelet Node Allocatable for kubepods: min(pid_max, threads-max) - reserved, floored at zero.
func expectedPidsMax(pidMax, threadsMax, reserved uint64) string {
	capacity := min(pidMax, threadsMax)

	if capacity <= reserved {
		return "0"
	}

	return strconv.FormatUint(capacity-reserved, 10)
}

func workloadMemoryRoot(percent uint64) *runtimecfg.WorkloadResourceRoot {
	return &runtimecfg.WorkloadResourceRoot{
		MemoryConfig: &runtimecfg.WorkloadMemoryResource{
			MemoryLimit: meta.MustPercentageSize(strconv.FormatUint(percent, 10) + "%"),
		},
	}
}

func init() {
	allSuites = append(allSuites, new(CGroupsSuite))
}
