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
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

	stream, err := suite.Client.MachineClient.List(ctx, &machineapi.ListRequest{Root: constants.CgroupMountPath})
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

// TestWorkloadMemoryLimits verifies the Talos-owned root limit lifecycle.
func (suite *CGroupsSuite) TestWorkloadMemoryLimits() {
	if testing.Short() {
		suite.T().Skip("skipping machine configuration changes in short mode")
	}

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("cgroups are nested in container mode, so the workload roots are not managed")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	original, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if original.WorkloadResourceConfig() != nil {
		suite.T().Skipf("node %s already carries a %s document", node, runtimecfg.WorkloadResourceConfigKind)
	}

	originalBytes, err := original.Bytes()
	suite.Require().NoError(err)

	suite.Require().Equal("max", suite.readMemoryMax(nodeCtx, constants.CgroupTalosContainersRoot))
	suite.Require().Equal("max", suite.readMemoryMax(nodeCtx, constants.CgroupVirtualMachines))

	memory, err := suite.Client.Memory(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().Len(memory.GetMessages(), 1)

	memTotal := memory.GetMessages()[0].GetMeminfo().GetMemtotal() * 1024
	suite.Require().NotZero(memTotal)

	// MiB-aligned limits are exact on both supported architectures, irrespective of base page size.
	const alignment = uint64(1 << 20)

	virtualMachinesLimit := memTotal / 8 / alignment * alignment
	suite.Require().NotZero(virtualMachinesLimit)

	containersLimit := virtualMachinesLimit * 2

	for _, root := range []string{constants.CgroupTalosContainersRoot, constants.CgroupVirtualMachines} {
		events := suite.ReadFile(nodeCtx, filepath.Join(constants.CgroupMountPath, root, "cgroup.events"))
		if !slices.Contains(strings.Split(events, "\n"), "populated 0") {
			suite.T().Skipf("workload root %s is populated", root)
		}

		current, readErr := strconv.ParseUint(suite.ReadFile(nodeCtx, filepath.Join(constants.CgroupMountPath, root, "memory.current")), 10, 64)
		suite.Require().NoError(readErr)

		if current >= virtualMachinesLimit {
			suite.T().Skipf("workload root %s retains %d bytes, exceeding the test budget", root, current)
		}
	}

	suite.T().Logf("node %s MemTotal %d: limiting %s to %d and %s to %d",
		node, memTotal, constants.CgroupTalosContainersRoot, containersLimit, constants.CgroupVirtualMachines, virtualMachinesLimit)

	// Register the restore before the first apply: a failed assertion must not leave the limit on
	// the shared cluster. The test deadline may have expired by then.
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

		suite.assertWorkloadMemoryMax(cleanupCtx, "max", "max")
	})

	doc := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	doc.TalosContainersConfig = workloadMemoryRoot(containersLimit)
	doc.VirtualMachinesConfig = workloadMemoryRoot(virtualMachinesLimit)

	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, runtime.WorkloadMemorySpecID,
		func(spec *runtime.WorkloadMemorySpec, asrt *assert.Assertions) {
			asrt.Equal(containersLimit, spec.TypedSpec().TalosContainersLimit)
			asrt.Equal(virtualMachinesLimit, spec.TypedSpec().VirtualMachinesLimit)
		})

	suite.assertWorkloadMemoryMax(nodeCtx, strconv.FormatUint(containersLimit, 10), strconv.FormatUint(virtualMachinesLimit, 10))

	// Lowering one root and dropping the other: the dropped root must return to unlimited. The
	// document is replaced rather than merged, as a merge patch would keep the dropped root.
	doc.TalosContainersConfig = workloadMemoryRoot(containersLimit / 2)
	doc.VirtualMachinesConfig = nil

	suite.RemoveMachineConfigDocuments(nodeCtx, runtimecfg.WorkloadResourceConfigKind)
	suite.PatchMachineConfig(nodeCtx, doc)

	suite.assertWorkloadMemoryMax(nodeCtx, strconv.FormatUint(containersLimit/2, 10), "max")

	suite.RemoveMachineConfigDocuments(nodeCtx, runtimecfg.WorkloadResourceConfigKind)

	suite.assertWorkloadMemoryMax(nodeCtx, "max", "max")
}

func (suite *CGroupsSuite) readMemoryMax(nodeCtx context.Context, name string) string {
	return suite.ReadFile(nodeCtx, filepath.Join(constants.CgroupMountPath, name, "memory.max"))
}

func (suite *CGroupsSuite) assertWorkloadMemoryMax(nodeCtx context.Context, talosContainers, virtualMachines string) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		for name, expected := range map[string]string{
			constants.CgroupTalosContainersRoot: talosContainers,
			constants.CgroupVirtualMachines:     virtualMachines,
		} {
			reader, err := suite.Client.Read(nodeCtx, filepath.Join(constants.CgroupMountPath, name, "memory.max"))
			if !asrt.NoError(err) {
				continue
			}

			body, err := io.ReadAll(reader)
			closeErr := reader.Close()

			if !asrt.NoError(err) || !asrt.NoError(closeErr) {
				continue
			}

			asrt.Equal(expected, strings.TrimSpace(string(body)), "%s memory.max", name)
		}
	}, time.Minute, time.Second, "workload memory limits should be applied")
}

func workloadMemoryRoot(limit uint64) *runtimecfg.WorkloadResourceRoot {
	return &runtimecfg.WorkloadResourceRoot{
		MemoryConfig: &runtimecfg.WorkloadMemoryResource{
			MemoryLimit: meta.MustByteSize(strconv.FormatUint(limit, 10)),
		},
	}
}

func init() {
	allSuites = append(allSuites, new(CGroupsSuite))
}
