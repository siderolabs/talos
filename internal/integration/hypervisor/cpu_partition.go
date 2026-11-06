// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// cpuPartitionMinCPUs splits a node into the host roots, a shared virtual machine pool, a plain
// slice and an exclusive slice, one CPU each at least.
const cpuPartitionMinCPUs = 4

const (
	cpuPartitionExclusiveSlice = "database"
	cpuPartitionPlainSlice     = "batch"
)

// TestCPUPartitionSlices applies a CPU partition policy with an exclusive virtual machine slice,
// reboots to make it the policy of the boot, and checks where virtual machines actually run.
func (suite *LibvirtSuite) TestCPUPartitionSlices() {
	if testing.Short() {
		suite.T().Skip("skipping reboots in short mode")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	online := suite.ReadOnlineCPUs(nodeCtx)
	if online.Size() < cpuPartitionMinCPUs {
		suite.T().Skipf("node %s has %d online CPUs, the test needs at least %d", node, online.Size(), cpuPartitionMinCPUs)
	}

	// Initial setup refuses to take over a virtual machine root which already runs tasks.
	suite.RequireWorkloadRootIdle(nodeCtx, constants.CgroupVirtualMachines)

	cpus := online.List()
	last := len(cpus) - 1
	host := cpuset.New(cpus[0])
	vmRoot := cpuset.New(cpus[1:]...)
	exclusive := cpuset.New(cpus[last])
	plain := cpuset.New(cpus[last-1])
	shared := vmRoot.Difference(exclusive).Difference(plain)

	originalBytes := suite.RequireNoCPUPartitionConfig(nodeCtx, node)
	suite.RestoreCPUPartitionOnCleanup(node, originalBytes)

	suite.T().Logf("node %s online CPUs %q: host %q, virtual machines %q, exclusive %q, plain %q, shared %q",
		node, online, host, vmRoot, exclusive, plain, shared)

	suite.PatchMachineConfig(nodeCtx, cpuPartitionVirtualMachinesDocument(host, vmRoot, exclusive, plain))
	suite.RebootForCPUPartition(suite.ctx, node)

	exclusivePath := constants.CgroupVirtualMachines + "/" + cpuPartitionExclusiveSlice + ".partition"
	plainPath := constants.CgroupVirtualMachines + "/" + cpuPartitionPlainSlice + ".partition"
	sharedPath := constants.CgroupVirtualMachines + "/" + runtimecfg.CPUPartitionSharedSliceName + ".partition"

	// kubepods is bounded to the host CPU, so every other online CPU is reserved for the system.
	suite.Require().Equal(runtime.CPUPartitionStatusSpec{
		Ready:               true,
		KubeletReservedCPUs: online.Difference(host).String(),
		VirtualMachinePools: []runtime.CPUPartitionPoolSpec{
			{Name: runtimecfg.CPUPartitionSharedSliceName, CPUs: shared.String(), Partition: "/" + sharedPath},
			{Name: cpuPartitionPlainSlice, CPUs: plain.String(), Partition: "/" + plainPath},
			{Name: cpuPartitionExclusiveSlice, CPUs: exclusive.String(), Partition: "/" + exclusivePath},
		},
	}, *suite.AssertCPUPartitionReady(nodeCtx))

	// The exclusive slice is a remote partition root; the virtual machine root above it stays a
	// member, delegating the CPUs and keeping only the rest.
	suite.AssertCgroupFile(nodeCtx, exclusivePath, "cpuset.cpus.partition", "root")
	suite.AssertCgroupCPUs(nodeCtx, exclusivePath, "cpuset.cpus.exclusive.effective", exclusive)
	suite.AssertCgroupCPUs(nodeCtx, exclusivePath, "cpuset.cpus.effective", exclusive)
	suite.AssertCgroupFile(nodeCtx, constants.CgroupVirtualMachines, "cpuset.cpus.partition", "member")
	suite.AssertCgroupCPUs(nodeCtx, constants.CgroupVirtualMachines, "cpuset.cpus", vmRoot)
	suite.AssertCgroupCPUs(nodeCtx, constants.CgroupVirtualMachines, "cpuset.cpus.exclusive", exclusive)
	suite.AssertCgroupCPUs(nodeCtx, constants.CgroupVirtualMachines, "cpuset.cpus.effective", vmRoot.Difference(exclusive))
	suite.AssertCgroupFile(nodeCtx, plainPath, "cpuset.cpus.partition", "member")
	suite.AssertCgroupCPUs(nodeCtx, plainPath, "cpuset.cpus.effective", plain)
	suite.AssertCgroupCPUs(nodeCtx, sharedPath, "cpuset.cpus.effective", shared)

	for _, root := range []string{constants.CgroupInit, constants.CgroupSystem, constants.CgroupPodRuntimeRoot, constants.CgroupTalosContainersRoot} {
		suite.AssertCgroupCPUs(nodeCtx, root, "cpuset.cpus.effective", host)
	}

	sliced := suite.cpuPartitionGuest(node, "vm-cpu-sliced-"+uuid.NewString(), cpuPartitionExclusiveSlice)
	unsliced := suite.cpuPartitionGuest(node, "vm-cpu-shared-"+uuid.NewString(), "")

	slicedPID := suite.assertGuestInPartition(nodeCtx, sliced.Name(), exclusivePath)
	unslicedPID := suite.assertGuestInPartition(nodeCtx, unsliced.Name(), sharedPath)

	// Moving one machine to another pool replaces that machine only.
	sliced.CPUConfig.CPUSlice = cpuPartitionPlainSlice
	suite.PatchMachineConfig(nodeCtx, sliced)

	movedPID := suite.assertGuestInPartition(nodeCtx, sliced.Name(), plainPath)
	suite.Require().NotEqual(slicedPID, movedPID, "guest %q was not replaced", sliced.Name())

	pid, err := suite.alpineGuestQEMUPID(nodeCtx, unsliced.Name())
	suite.Require().NoError(err)
	suite.Require().Equal(unslicedPID, pid, "guest %q was replaced though its placement did not change", unsliced.Name())
}

func cpuPartitionVirtualMachinesDocument(host, vmRoot, exclusive, plain cpuset.CPUSet) *runtimecfg.CPUPartitionConfigV1Alpha1 {
	hostRoot := func() *runtimecfg.CPUPartitionRoot { return &runtimecfg.CPUPartitionRoot{RootCPUs: host.String()} }

	// An exclusive slice requires every fixed root to be bounded away from it.
	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.InitConfig = hostRoot()
	doc.SystemConfig = hostRoot()
	doc.PodRuntimeConfig = hostRoot()
	doc.KubepodsConfig = hostRoot()
	doc.TalosContainersConfig = hostRoot()
	doc.VirtualMachinesConfig = &runtimecfg.CPUPartitionVirtualMachines{
		RootCPUs: vmRoot.String(),
		SlicesConfig: []runtimecfg.CPUPartitionSlice{
			{SliceName: cpuPartitionExclusiveSlice, SliceCPUs: exclusive.String(), SliceExclusive: new(true)},
			{SliceName: cpuPartitionPlainSlice, SliceCPUs: plain.String()},
		},
	}

	return doc
}

// cpuPartitionGuest starts a diskless guest in the slice, removed again on cleanup.
func (suite *LibvirtSuite) cpuPartitionGuest(node, name, slice string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.CPUConfig.CPUSlice = slice
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(client.WithNode(suite.ctx, node), doc)
	suite.assertRunningTransientDomain(node, name, 1)

	return doc
}

// assertGuestInPartition waits until the guest's QEMU process runs below the partition and returns its PID.
func (suite *LibvirtSuite) assertGuestInPartition(nodeCtx context.Context, name, partition string) int32 {
	suite.T().Helper()

	var pid int32

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		var err error

		pid, err = suite.alpineGuestQEMUPID(nodeCtx, name)
		if !assert.NoError(collect, err) || !assert.NotZero(collect, pid, "no QEMU process for guest %q", name) {
			return
		}

		reader, err := suite.Client.Read(nodeCtx, fmt.Sprintf("/proc/%d/cgroup", pid))
		if !assert.NoError(collect, err) {
			return
		}

		body, err := io.ReadAll(reader)
		closeErr := reader.Close()

		if !assert.NoError(collect, err) || !assert.NoError(collect, closeErr) {
			return
		}

		assert.Contains(collect, string(body), "0::/"+partition+"/", "QEMU %d of guest %q runs in %s", pid, name, body)
	}, 2*time.Minute, time.Second, "guest %q should run in %s", name, partition)

	return pid
}
