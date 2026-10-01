// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	sharedPartition   = "/virtualmachines.partition/shared.partition"
	databasePartition = "/virtualmachines.partition/database.partition"
	runtimeFinalizer  = "hypervisor.VirtualMachineController"
)

// A virtual-machine-managing policy: the guard is on.
func managingSpec() *runtimeres.CPUPartitionSpec {
	spec := runtimeres.NewCPUPartitionSpec()
	spec.TypedSpec().Enabled = true
	spec.TypedSpec().Roots = map[string]string{"virtualMachines": "4-7"}
	spec.TypedSpec().Slices = []runtimeres.CPUPartitionSliceSpec{{Name: "database", CPUs: "4-5", Exclusive: true}}

	return spec
}

func appliedStatus() *runtimeres.CPUPartitionStatus {
	status := runtimeres.NewCPUPartitionStatus()
	status.TypedSpec().Phase = runtimeres.CPUPartitionPhaseReady
	status.TypedSpec().Targets = []runtimeres.CPUPartitionTargetStatus{
		{Key: "virtualMachines", LastApplied: "4-7"},
		{Key: "virtualMachines/database", LastApplied: "4-5"},
		{Key: "virtualMachines/shared", LastApplied: "6-7"},
	}
	status.TypedSpec().Exclusive = []string{"database"}

	return status
}

func placement(name, partition, slice string, exclusive bool) *hypervisor.VirtualMachineCPUPlacement {
	res := hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, name)
	res.TypedSpec().Partition = partition
	res.TypedSpec().Slice = slice
	res.TypedSpec().Exclusive = exclusive

	return res
}

func domainXML(name, partition, pins string) string {
	xml := `<domain type="kvm"><name>` + name + `</name><vcpu>1</vcpu><resource><partition>` + partition + `</partition></resource>`
	if pins != "" {
		xml += `<cputune><vcpupin vcpu="0" cpuset="` + pins + `"/></cputune>`
	}

	return xml + `</domain>`
}

func (s *VirtualMachineDomainSuite) replaceSpec(spec *runtimeres.CPUPartitionSpec) {
	existing, err := safe.StateGetByID[*runtimeres.CPUPartitionSpec](s.Ctx(), s.State(), runtimeres.CPUPartitionSpecID)
	s.Require().NoError(err)

	spec.Metadata().SetVersion(existing.Metadata().Version())
	s.Update(spec)
}

func (s *VirtualMachineDomainSuite) starts(name string) int {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	return s.client.starts[name]
}

func (s *VirtualMachineDomainSuite) placementHeld(name string) bool {
	res, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), name)

	return err == nil && res.Metadata().Finalizers().Has(runtimeFinalizer)
}

// Projection pending: a running spec must not start. Once the (disabled) projection arrives the
// domain starts in the root partition as before, without any placement.
func (s *VirtualMachineDomainSuite) TestProjectionPendingBlocksStartOnly() {
	existing, err := safe.StateGetByID[*runtimeres.CPUPartitionSpec](s.Ctx(), s.State(), runtimeres.CPUPartitionSpecID)
	s.Require().NoError(err)
	s.Destroy(existing)

	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "plain")
	spec.TypedSpec().DomainXML = domainXML("plain", "/virtualmachines.partition", "")
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	select {
	case <-s.client.attempted:
		s.FailNow("a domain was started while the CPU partition projection was pending")
	case <-time.After(100 * time.Millisecond):
	}

	s.Create(runtimeres.NewCPUPartitionSpec())
	s.assertDomain("plain", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("plain", true)
	s.Require().False(s.placementHeld("plain"))
}

// Under a managing policy a domain starts only with a granted placement matching its definition,
// with the placement claimed before the start and released after the domain is removed.
func (s *VirtualMachineDomainSuite) TestGuardedStartClaimsPlacementFirst() {
	s.replaceSpec(managingSpec())
	s.Create(appliedStatus())
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "db")
	spec.TypedSpec().DomainXML = domainXML("db", databasePartition, "4")
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	select {
	case <-s.client.attempted:
		s.FailNow("a domain was started without a placement")
	case <-time.After(100 * time.Millisecond):
	}

	// A placement for a different partition than the definition carries is not enough either.
	s.Create(placement("db", sharedPartition, "", false))

	select {
	case <-s.client.attempted:
		s.FailNow("a domain was started with a placement not matching its definition")
	case <-time.After(100 * time.Millisecond):
	}

	ctest.UpdateWithConflicts(s, placement("db", sharedPartition, "", false), func(res *hypervisor.VirtualMachineCPUPlacement) error {
		res.TypedSpec().Partition = databasePartition
		res.TypedSpec().Slice = "database"
		res.TypedSpec().Exclusive = true

		return nil
	})

	s.assertDomain("db", spec.TypedSpec().DomainXML, true)
	s.assertFinalizer("db", true)
	s.Require().True(s.placementHeld("db"))

	// Stop: the domain is removed, then both claims go.
	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().PowerState = "stopped"

		return nil
	})
	s.assertDomain("db", "", false)
	s.assertFinalizer("db", false)
	s.Require().Eventually(func() bool { return !s.placementHeld("db") }, 5*time.Second, 10*time.Millisecond)
}

// Pins outside the applied CPUs of the placement are refused before libvirt is asked.
func (s *VirtualMachineDomainSuite) TestGuardedStartRefusesPinsOutsideAllocation() {
	s.replaceSpec(managingSpec())
	s.Create(appliedStatus())
	s.Create(placement("db", databasePartition, "database", true))
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "db")
	spec.TypedSpec().DomainXML = domainXML("db", databasePartition, "6")
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	select {
	case <-s.client.attempted:
		s.FailNow("a domain with pins outside its slice was started")
	case <-time.After(100 * time.Millisecond):
	}

	s.Require().Zero(s.starts("db"))
	// A refused start leaves no claim behind: nothing was started.
	s.Require().False(s.placementHeld("db"))
	s.assertFinalizer("db", false)
}

// A stale placement whose partition the applied policy does not (yet) verify, a pending intent,
// or a detected enforcement loss all deny a new start; the claim is still taken as evidence.
func (s *VirtualMachineDomainSuite) TestGuardedStartRequiresVerifiedAllocation() {
	s.replaceSpec(managingSpec())

	status := appliedStatus()
	status.TypedSpec().Targets = []runtimeres.CPUPartitionTargetStatus{{Key: "virtualMachines", LastApplied: "4-7"}}
	s.Create(status)
	s.Create(placement("db", databasePartition, "database", true))
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "db")
	spec.TypedSpec().DomainXML = domainXML("db", databasePartition, "")
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)

	s.assertNoStart("no applied target for the placement's partition")

	// A write in flight is not an allocation either.
	ctest.UpdateWithConflicts(s, status, func(res *runtimeres.CPUPartitionStatus) error {
		res.TypedSpec().SetTarget(runtimeres.CPUPartitionTargetStatus{Key: "virtualMachines/database", LastApplied: "", Intended: "4-5"})

		return nil
	})
	s.assertNoStart("a pending intent")

	// Verified, but a boundary lost enforcement: closed.
	ctest.UpdateWithConflicts(s, status, func(res *runtimeres.CPUPartitionStatus) error {
		res.TypedSpec().SetTarget(runtimeres.CPUPartitionTargetStatus{Key: "virtualMachines/database", LastApplied: "4-5", Intended: "4-5"})
		res.TypedSpec().EnforcementLoss = []string{"init"}

		return nil
	})
	s.assertNoStart("an enforcement loss")

	// Admission closed by the coordinator: still no start.
	ctest.UpdateWithConflicts(s, status, func(res *runtimeres.CPUPartitionStatus) error {
		res.TypedSpec().EnforcementLoss = nil
		res.TypedSpec().Phase = runtimeres.CPUPartitionPhaseApplying

		return nil
	})
	s.assertNoStart("a closed admission")

	ctest.UpdateWithConflicts(s, status, func(res *runtimeres.CPUPartitionStatus) error {
		res.TypedSpec().Phase = runtimeres.CPUPartitionPhaseReady

		return nil
	})
	s.assertDomain("db", spec.TypedSpec().DomainXML, true)
}

func (s *VirtualMachineDomainSuite) assertNoStart(reason string) {
	s.T().Helper()

	select {
	case <-s.client.attempted:
		s.FailNow("a domain was started despite " + reason)
	case <-time.After(100 * time.Millisecond):
	}
}

// A placement being torn down is not granted for a new start, but the domain it already holds
// keeps running and the user-requested stop still flows through and releases it.
func (s *VirtualMachineDomainSuite) TestWithdrawnPlacementKeepsRunningDomainUntilUserStops() {
	s.replaceSpec(managingSpec())
	s.Create(appliedStatus())
	s.Create(placement("web", sharedPartition, "", false))
	s.start()

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "web")
	spec.TypedSpec().DomainXML = domainXML("web", sharedPartition, "")
	spec.TypedSpec().PowerState = "running"
	s.Create(spec)
	s.assertDomain("web", spec.TypedSpec().DomainXML, true)
	s.Require().True(s.placementHeld("web"))

	// The coordinator withdraws the placement (policy removal): the claim stops the destroy.
	ready, err := s.State().Teardown(s.Ctx(), placement("web", sharedPartition, "", false).Metadata())
	s.Require().NoError(err)
	s.Require().False(ready)

	select {
	case <-s.client.attemptedRemove:
		s.FailNow("a running domain was removed because its placement was withdrawn")
	case <-time.After(100 * time.Millisecond):
	}

	s.assertDomain("web", spec.TypedSpec().DomainXML, true)
	s.Require().Equal(1, s.starts("web"))

	// A new start is refused while the placement is withdrawn ...
	other := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "other")
	other.TypedSpec().DomainXML = domainXML("other", sharedPartition, "")
	other.TypedSpec().PowerState = "running"

	s.Create(placement("other", sharedPartition, "", false))

	ready, err = s.State().Teardown(s.Ctx(), placement("other", sharedPartition, "", false).Metadata())
	s.Require().NoError(err)
	s.Require().True(ready)

	// Drain the signal of the earlier start before watching for a new attempt.
	select {
	case <-s.client.attempted:
	default:
	}

	s.Create(other)

	select {
	case <-s.client.attempted:
		s.FailNow("a domain was started with a withdrawn placement")
	case <-time.After(100 * time.Millisecond):
	}

	s.Require().Zero(s.starts("other"))

	// ... but the operator's stop goes through and releases the claim.
	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().PowerState = "stopped"

		return nil
	})
	s.assertDomain("web", "", false)
	s.Require().Eventually(func() bool {
		res, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), "web")

		return err == nil && !res.Metadata().Finalizers().Has(runtimeFinalizer) && res.Metadata().Phase() == resource.PhaseTearingDown
	}, 5*time.Second, 10*time.Millisecond)
}

// The runtime never re-defines a running domain because the policy changed: with an identical
// definition, policy and status churn produce no Start call, so the domain identity survives.
func TestVirtualMachineLiveMaskChangeDoesNotRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &VirtualMachineDomainSuite{}
		s.SetT(t)
		s.SetupTest()

		defer s.TearDownTest()

		s.replaceSpec(managingSpec())

		status := appliedStatus()
		s.Create(status)
		s.Create(placement("db", databasePartition, "database", true))
		s.start()

		spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "db")
		spec.TypedSpec().DomainXML = domainXML("db", databasePartition, "4")
		spec.TypedSpec().PowerState = "running"
		s.Create(spec)

		synctest.Wait()

		s.Require().Equal(1, s.starts("db"))
		s.Require().True(s.placementHeld("db"))

		// The slice shrinks live (4-5 -> 4): status and spec change, the definition does not.
		ctest.UpdateWithConflicts(s, status, func(res *runtimeres.CPUPartitionStatus) error {
			res.TypedSpec().SetTarget(runtimeres.CPUPartitionTargetStatus{Key: "virtualMachines/database", LastApplied: "4"})

			return nil
		})

		shrunk := managingSpec()
		shrunk.TypedSpec().Slices[0].CPUs = "4"

		s.replaceSpec(shrunk)

		synctest.Wait()

		s.Require().Equal(1, s.starts("db"), "a live mask change must not restart the domain")
		s.Require().True(s.placementHeld("db"))
	})
}
