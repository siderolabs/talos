// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

type placementFixture struct {
	VirtualMachineDomainSuite
}

func TestVirtualMachinePlacement(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*placementFixture)
	}{
		{"waits for topology", (*placementFixture).explicitPlacementWaitsForTopology},
		{"rejects invalid placement", (*placementFixture).invalidHostPlacementIsRejected},
		{"preserves running domain", (*placementFixture).runningDomainSurvivesTopologyChange},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := &placementFixture{}
				s.SetT(t)
				s.SetupTest()
				t.Cleanup(s.TearDownTest)
				test.run(s)
			})
		})
	}
}

func newNUMATopology() *hardware.NUMATopology {
	topology := hardware.NewNUMATopology()
	*topology.TypedSpec() = hardware.NUMATopologySpec{
		Nodes: []hardware.NUMANodeSpec{
			{ID: 0, CPUs: []uint32{0, 1, 2, 3}, MemoryTotalBytes: 1 << 30},
			{ID: 1, CPUs: []uint32{4, 5}, MemoryTotalBytes: 1 << 30},
			{ID: 2, MemoryTotalBytes: 1 << 30},
			{ID: 3, CPUs: []uint32{6}},
		},
		PresentCPUs: []uint32{0, 1, 2, 3, 4, 5, 6},
		OnlineCPUs:  []uint32{0, 1, 2, 4, 5, 6},
	}

	return topology
}

func placedDomainXML(name, cpus, nodes string) string {
	return `<domain><name>` + name + `</name><vcpu>1</vcpu><cputune><vcpupin vcpu="0" cpuset="` + cpus +
		`"/></cputune><numatune><memory mode="strict" nodeset="` + nodes + `"/></numatune></domain>`
}

func (s *placementFixture) newRunningDomainSpec(name, domainXML string) *hypervisor.VirtualMachineDomainSpec {
	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().DomainXML = domainXML
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	return spec
}

func (s *placementFixture) assertNotStarted(name string) {
	client := s.client

	synctest.Wait()
	client.mu.Lock()
	defer client.mu.Unlock()

	_, started := client.domains[name]
	s.Require().False(started)
	s.Require().Zero(client.starts[name])
}

func (s *placementFixture) explicitPlacementWaitsForTopology() {
	placed := s.newRunningDomainSpec("placed", placedDomainXML("placed", "0", "0"))
	plain := s.newRunningDomainSpec("plain", `<domain><name>plain</name><vcpu>1</vcpu></domain>`)
	s.start()

	s.assertDomain("plain", plain.TypedSpec().DomainXML, true)
	s.assertNotStarted("placed")

	s.Create(newNUMATopology())
	s.assertDomain("placed", placed.TypedSpec().DomainXML, true)
}

func (s *placementFixture) invalidHostPlacementIsRejected() {
	s.Create(newNUMATopology())

	for _, test := range []struct{ name, cpus, nodes string }{
		{name: "offline-cpu", cpus: "3", nodes: "0"},
		{name: "absent-cpu", cpus: "7", nodes: "0"},
		{name: "absent-node", cpus: "0", nodes: "4"},
		{name: "memoryless-node", cpus: "0", nodes: "3"},
	} {
		s.newRunningDomainSpec(test.name, placedDomainXML(test.name, test.cpus, test.nodes))
	}

	s.start()

	// Intentional remote memory: vCPU on node 1, memory on the CPUless node 2.
	remote := s.newRunningDomainSpec("remote", placedDomainXML("remote", "4", "2"))
	s.assertDomain("remote", remote.TypedSpec().DomainXML, true)

	for _, name := range []string{"offline-cpu", "absent-cpu", "absent-node", "memoryless-node"} {
		s.assertNotStarted(name)
	}

	// A rejection is not a controller failure: a failed controller is restarted with backoff and
	// reconciles again with no new event, opening another session.
	client := s.client
	client.mu.Lock()
	opens := client.opens
	client.mu.Unlock()

	synctest.Sleep(2 * time.Second)
	synctest.Wait()
	client.mu.Lock()
	observedOpens := client.opens
	client.mu.Unlock()
	s.Require().Equal(opens, observedOpens)

	fixed := placedDomainXML("offline-cpu", "2", "0")

	ctest.UpdateWithConflicts(s, hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "offline-cpu"),
		func(spec *hypervisor.VirtualMachineDomainSpec) error {
			spec.TypedSpec().DomainXML = fixed

			return nil
		})
	s.assertDomain("offline-cpu", fixed, true)
}

// An unchanged running domain is never stopped because the inventory changed under it; a
// replacement whose placement became invalid keeps the old domain running.
func (s *placementFixture) runningDomainSurvivesTopologyChange() {
	topology := newNUMATopology()
	s.Create(topology)

	original := placedDomainXML("vm", "1", "0")
	s.newRunningDomainSpec("vm", original)
	s.start()
	s.assertDomain("vm", original, true)

	ctest.UpdateWithConflicts(s, topology, func(current *hardware.NUMATopology) error {
		current.TypedSpec().OnlineCPUs = []uint32{0, 2, 4, 5, 6}

		return nil
	})

	replacement := placedDomainXML("vm", "1", "1")

	ctest.UpdateWithConflicts(s, hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm"),
		func(spec *hypervisor.VirtualMachineDomainSpec) error {
			spec.TypedSpec().DomainXML = replacement

			return nil
		})

	client := s.client

	synctest.Wait()
	client.mu.Lock()
	domain, running := client.domains["vm"]
	text, starts := client.texts["vm"], client.starts["vm"]
	client.mu.Unlock()
	s.Require().True(running)
	s.Require().Equal(libvirtdomain.UUID(uuid.MustParse(machineUUID), "vm"), domain.UUID)
	s.Require().Equal(original, text)
	s.Require().Equal(1, starts)

	ctest.UpdateWithConflicts(s, topology, func(current *hardware.NUMATopology) error {
		current.TypedSpec().OnlineCPUs = []uint32{0, 1, 2, 4, 5, 6}

		return nil
	})
	s.assertDomain("vm", replacement, true)
}

func newPlacedSpec(name, cpus, nodes string) *hypervisor.VirtualMachineSpec {
	spec := newRenderableSpec(name, "running")
	spec.TypedSpec().CPU.Pins = []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: cpus}}
	spec.TypedSpec().Memory.NUMA = &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "strict", Nodes: nodes}

	return spec
}

// Placement waiting for the inventory is pending; host IDs the host cannot use are an error naming
// the ID; the observed power state of a running domain is kept either way.
func (s *VirtualMachineStatusSuite) TestHostPlacementStatus() {
	name := "vm1"
	s.client.domains[name] = libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), name)}

	s.Create(newPlacedSpec(name, "3", "0"))
	s.Create(newPlacedSpec("vm2", "4", "2"))
	s.Create(newRenderableSpec("plain", "running"))
	s.start()

	s.assertStatus(name, "running", hypervisor.VirtualMachineStagePending,
		`virtual machine "vm1": explicit host placement: host NUMA topology is not available yet`)
	s.assertStatus("vm2", "unknown", hypervisor.VirtualMachineStagePending,
		`virtual machine "vm2": explicit host placement: host NUMA topology is not available yet`)
	s.assertStatus("plain", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")

	s.Create(newNUMATopology())
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageError,
		`virtual machine "vm1": invalid host placement: vCPU 0 pin names offline host CPU 3`)
	// Remote memory on a CPUless node is a valid selection.
	s.assertStatus("vm2", "unknown", hypervisor.VirtualMachineStageUnknown, "domain has not been observed")

	ctest.UpdateWithConflicts(s, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name),
		func(spec *hypervisor.VirtualMachineSpec) error {
			spec.TypedSpec().Memory.NUMA.Nodes = "3"
			spec.TypedSpec().CPU.Pins[0].CPUs = "6"

			return nil
		})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageError,
		`virtual machine "vm1": invalid host placement: memory nodeset names host NUMA node 3 without memory`)

	ctest.UpdateWithConflicts(s, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name),
		func(spec *hypervisor.VirtualMachineSpec) error {
			spec.TypedSpec().Memory.NUMA.Nodes = "2"

			return nil
		})
	s.assertStatus(name, "running", hypervisor.VirtualMachineStageReady, "")
}
