// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"runtime"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// cpuPartitionReady publishes the status of a boot without a CPU partition policy, which every
// domain waits for.
func cpuPartitionReady(s ctest.Suite) {
	status := runtimeres.NewCPUPartitionStatus()
	status.TypedSpec().Ready = true
	s.Require().NoError(s.State().Create(s.Ctx(), status))
}

// setCPUPartition replaces the status published by cpuPartitionReady.
func setCPUPartition(s ctest.Suite, spec runtimeres.CPUPartitionStatusSpec) {
	ctest.UpdateWithConflicts(s, runtimeres.NewCPUPartitionStatus(), func(res *runtimeres.CPUPartitionStatus) error {
		*res.TypedSpec() = spec

		return nil
	})
}

func twoSliceStatus() runtimeres.CPUPartitionStatusSpec {
	return runtimeres.CPUPartitionStatusSpec{
		Ready: true,
		VirtualMachinePools: []runtimeres.CPUPartitionPoolSpec{
			{Name: "shared", CPUs: "4", Partition: "/virtualmachines.partition/shared.partition"},
			{Name: "batch", CPUs: "5", Partition: "/virtualmachines.partition/batch.partition"},
			{Name: "database", CPUs: "6-7", Partition: "/virtualmachines.partition/database.partition"},
		},
	}
}

func (s *VirtualMachineDomainSuite) startCPUManaged() {
	if runtime.GOARCH != "amd64" {
		s.T().Skip("builtin VM profiles are only supported on amd64")
	}

	cpus := []uint32{0, 1, 2, 3, 4, 5, 6, 7}

	topology := hardware.NewNUMATopology()
	*topology.TypedSpec() = hardware.NUMATopologySpec{
		Nodes:       []hardware.NUMANodeSpec{{ID: 0, CPUs: cpus, MemoryTotalBytes: 1 << 30}},
		PresentCPUs: cpus,
		OnlineCPUs:  cpus,
	}
	s.Create(topology)

	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.start()
}

func (s *VirtualMachineDomainSuite) createVM(name, slice string) *hypervisor.VirtualMachineSpec {
	vm := newRenderableSpec(name, "running")
	vm.TypedSpec().CPU.Slice = slice
	s.Create(vm)

	return vm
}

func (s *VirtualMachineDomainSuite) startCount(name string) int {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	return s.client.starts[name]
}

func (s *VirtualMachineDomainSuite) assertNoStart(name string) {
	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.starts[name] != 0
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func (s *VirtualMachineDomainSuite) assertRunning(name string, starts int, contains ...string) {
	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		_, ok := s.client.domains[name]

		for _, text := range contains {
			ok = ok && strings.Contains(s.client.texts[name], text)
		}

		return ok && s.client.starts[name] == starts
	}, 5*time.Second, 10*time.Millisecond)
}

func partitionXML(partition string) string {
	return "<partition>" + partition + "</partition>"
}

// Domains wait for the CPU partition policy of the boot and start in their pool once it is applied.
func (s *VirtualMachineDomainSuite) TestCPUPartitionGatesFirstStart() {
	setCPUPartition(s, runtimeres.CPUPartitionStatusSpec{})
	s.startCPUManaged()

	s.createVM("vm", "database")
	s.assertNoStart("vm")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](s, "vm")

	setCPUPartition(s, twoSliceStatus())
	s.assertRunning("vm", 1, partitionXML("/virtualmachines.partition/database.partition"))
}

// Changing the slice or the pins of one machine replaces that machine only.
func (s *VirtualMachineDomainSuite) TestPlacementChangeReplacesOnlyItsMachine() {
	setCPUPartition(s, twoSliceStatus())
	s.startCPUManaged()

	moving := s.createVM("moving", "database")
	pinned := s.createVM("pinned", "database")
	s.createVM("steady", "batch")
	s.assertRunning("moving", 1, partitionXML("/virtualmachines.partition/database.partition"))
	s.assertRunning("pinned", 1)
	s.assertRunning("steady", 1, partitionXML("/virtualmachines.partition/batch.partition"))

	ctest.UpdateWithConflicts(s, moving, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().CPU.Slice = ""

		return nil
	})

	ctest.UpdateWithConflicts(s, pinned, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().CPU.Pins = []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: "7"}}

		return nil
	})

	s.assertRunning("moving", 2, partitionXML("/virtualmachines.partition/shared.partition"))
	s.assertRunning("pinned", 2, `<vcpupin vcpu="0" cpuset="7">`)
	s.Require().Equal(1, s.startCount("steady"))
}

// A selection the boot policy cannot satisfy keeps the machine running on its last definition;
// an explicit stop still reaches it.
func (s *VirtualMachineDomainSuite) TestUnavailablePlacementKeepsRunningDomain() {
	setCPUPartition(s, twoSliceStatus())
	s.startCPUManaged()

	vm := s.createVM("vm", "database")
	s.assertRunning("vm", 1, partitionXML("/virtualmachines.partition/database.partition"))

	vm = ctest.UpdateWithConflicts(s, vm, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().CPU.Slice = "analytics"

		return nil
	})

	vm = ctest.UpdateWithConflicts(s, vm, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().CPU.Pins = []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: "0"}}

		return nil
	})

	client := s.client

	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return !strings.Contains(client.texts["vm"], "database.partition") || client.starts["vm"] != 1
	}, 300*time.Millisecond, 10*time.Millisecond)
	s.assertFinalizer("vm", true)

	ctest.UpdateWithConflicts(s, vm, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().PowerState = "stopped"

		return nil
	})

	s.assertDomain("vm", "", false)
	s.assertFinalizer("vm", false)
}

// Pins outside the selected pool keep a new machine from starting.
func (s *VirtualMachineDomainSuite) TestPinsOutsideThePoolAreRefused() {
	setCPUPartition(s, twoSliceStatus())
	s.startCPUManaged()

	vm := newRenderableSpec("outside", "running")
	vm.TypedSpec().CPU.Slice = "batch"
	vm.TypedSpec().CPU.Pins = []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: "6"}}
	s.Create(vm)

	s.assertNoStart("outside")

	ctest.UpdateWithConflicts(s, vm, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().CPU.Pins[0].CPUs = "5"

		return nil
	})

	s.assertRunning("outside", 1, partitionXML("/virtualmachines.partition/batch.partition"))
}

// Without a policy for the virtual machine root, domains keep the partition they had before CPU partitioning.
func (s *VirtualMachineDomainSuite) TestNoPolicyKeepsRootPartition() {
	s.startCPUManaged()

	s.createVM("vm", "")
	s.assertRunning("vm", 1, partitionXML("/virtualmachines.partition"))

	status, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")
	s.Require().NoError(err)
	s.Require().NotContains(status.TypedSpec().DomainXML, ".partition/")
}
