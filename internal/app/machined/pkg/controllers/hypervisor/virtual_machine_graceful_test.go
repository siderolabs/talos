// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"errors"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// startGracefully publishes a running domain spec which stops by asking its guest.
func (s *VirtualMachineDomainSuite) startGracefully(name string) *hypervisor.VirtualMachineDomainSpec {
	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().DomainXML = `<domain><name>` + name + `</name></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().StopMode = hypervisorhelpers.StopModeGraceful.String()
	s.Create(spec)
	s.assertDomain(name, spec.TypedSpec().DomainXML, true)
	s.assertFinalizer(name, true)

	return spec
}

// drivePowerState rewrites the power state a published domain spec is driven towards.
func (s *VirtualMachineDomainSuite) drivePowerState(spec *hypervisor.VirtualMachineDomainSpec, powerState, stopMode string) {
	ctest.UpdateWithConflicts(s, spec, func(res *hypervisor.VirtualMachineDomainSpec) error {
		res.TypedSpec().PowerState = powerState
		res.TypedSpec().StopMode = stopMode

		return nil
	})
}

// assertAsked waits for a guest to have been asked exactly count times, and then keeps checking,
// so that a controller which asks again on a later pass is caught rather than raced past.
func (s *VirtualMachineDomainSuite) assertAsked(name string, count int) {
	s.Require().Eventually(func() bool {
		return s.client.shutdownCount(name) == count
	}, 5*time.Second, 10*time.Millisecond, "guest was not asked to power off %d times", count)

	// Churn an unrelated resource so the controller reconciles again under the same state.
	s.churn()

	s.Require().Never(func() bool {
		return s.client.shutdownCount(name) != count
	}, 500*time.Millisecond, 10*time.Millisecond, "guest was asked more than %d times", count)
}

// churn wakes the controller without changing anything it acts on.
func (s *VirtualMachineDomainSuite) churn() {
	status := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "churn")
	s.Create(status)
	s.Destroy(status)
}

// awaitReconcile waits for a full reconciliation to have been carried out.
//
// Each one opens its own libvirt session, which is the only thing a pass that changes nothing
// leaves behind to observe.
func (s *VirtualMachineDomainSuite) awaitReconcile() {
	s.client.mu.Lock()
	before := s.client.opens
	s.client.mu.Unlock()

	s.churn()

	s.Require().Eventually(func() bool {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()

		return s.client.opens > before
	}, 5*time.Second, 10*time.Millisecond, "no reconciliation was carried out")
}

// powerOffGuest models a guest obeying the power button: the transient domain goes with it, and
// libvirt reports that as the domain status going away.
func (s *VirtualMachineDomainSuite) powerOffGuest(name string) {
	status := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, name)
	s.Create(status)

	s.client.powerOff(name)

	s.Destroy(status)
}

// A graceful stop asks the guest once, and asks nothing of the disks or the claim until the guest
// is actually gone: a guest which is powering itself off is still reading them.
func (s *VirtualMachineDomainSuite) TestGracefulStopAsksOnceAndKeepsItsHolds() {
	s.start()

	disk := s.newDiskStatus(diskStatusVM + "/install@aaaaaaaaaaaa")

	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, diskStatusVM)
	spec.TypedSpec().DomainXML = `<domain><name>` + diskStatusVM + `</name></domain>`
	spec.TypedSpec().PowerState = "running"
	spec.TypedSpec().StopMode = hypervisorhelpers.StopModeGraceful.String()
	spec.TypedSpec().Disks = []string{disk.Metadata().ID()}
	s.Create(spec)
	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
	s.assertDiskHeld(disk.Metadata().ID(), true)

	s.drivePowerState(spec, "stopped", hypervisorhelpers.StopModeGraceful.String())

	s.assertAsked(diskStatusVM, 1)

	// The guest is ignoring the power button, so nothing has moved.
	s.assertDomain(diskStatusVM, spec.TypedSpec().DomainXML, true)
	s.assertDiskHeld(disk.Metadata().ID(), true)
	s.assertFinalizer(diskStatusVM, true)

	s.powerOffGuest(diskStatusVM)

	s.assertDiskHeld(disk.Metadata().ID(), false)
	s.assertFinalizer(diskStatusVM, false)
	s.Require().Equal(1, s.client.shutdownCount(diskStatusVM))
}

// A forced stop, and a stop of a spec which names no mode at all, destroy the domain without
// asking its guest anything.
func (s *VirtualMachineDomainSuite) TestForcedStopNeverAsksTheGuest() {
	s.start()

	for _, stopMode := range []string{"", hypervisorhelpers.StopModeForced.String()} {
		name := "forced" + stopMode

		spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name)
		spec.TypedSpec().DomainXML = `<domain><name>` + name + `</name></domain>`
		spec.TypedSpec().PowerState = "running"
		spec.TypedSpec().StopMode = stopMode
		s.Create(spec)
		s.assertDomain(name, spec.TypedSpec().DomainXML, true)

		s.drivePowerState(spec, "stopped", stopMode)

		s.assertDomain(name, "", false)
		s.assertFinalizer(name, false)
		s.Require().Zero(s.client.shutdownCount(name))
	}
}

// A definition being withdrawn is not a stop the guest may refuse: holding the claim for as long
// as it liked would hold every disk with it.
func (s *VirtualMachineDomainSuite) TestTeardownIsAlwaysForced() {
	s.start()

	spec := s.startGracefully("teardown")

	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready, "the finalizer must prevent immediate deletion")

	s.assertDomain("teardown", "", false)
	s.assertFinalizer("teardown", false)
	s.Require().Zero(s.client.shutdownCount("teardown"))
}

// Switching to forced is the way out of a stop the guest is refusing.
func (s *VirtualMachineDomainSuite) TestForcedStopEscapesAGracefulOne() {
	s.start()

	spec := s.startGracefully("escape")

	s.drivePowerState(spec, "stopped", hypervisorhelpers.StopModeGraceful.String())
	s.assertAsked("escape", 1)
	s.assertDomain("escape", spec.TypedSpec().DomainXML, true)

	s.drivePowerState(spec, "stopped", hypervisorhelpers.StopModeForced.String())

	s.assertDomain("escape", "", false)
	s.assertFinalizer("escape", false)
}

// Driving a machine which is being asked to power off back to running leaves the guest alone: the
// press cannot be recalled, and a guest which refused it is healthy. A later stop asks again.
func (s *VirtualMachineDomainSuite) TestARunningGuestIsAskedAgainByTheNextStop() {
	s.start()

	spec := s.startGracefully("again")

	s.drivePowerState(spec, "stopped", hypervisorhelpers.StopModeGraceful.String())
	s.assertAsked("again", 1)

	s.drivePowerState(spec, "running", hypervisorhelpers.StopModeGraceful.String())
	s.awaitReconcile()

	// The definition did not change, so the guest which ignored the power button keeps running.
	s.assertDomain("again", spec.TypedSpec().DomainXML, true)

	s.client.mu.Lock()
	starts := s.client.starts["again"]
	s.client.mu.Unlock()
	s.Require().Equal(1, starts, "a domain which was never gone must not be started again")

	s.drivePowerState(spec, "stopped", hypervisorhelpers.StopModeGraceful.String())
	s.assertAsked("again", 2)
}

// A record of an ask does not outlive the spec it was made against, so a machine configured again
// under a name which was once stopped is still asked.
func (s *VirtualMachineDomainSuite) TestARecreatedSpecIsAskedAgain() {
	s.start()

	spec := s.startGracefully("recreated")

	s.drivePowerState(spec, "stopped", hypervisorhelpers.StopModeGraceful.String())
	s.assertAsked("recreated", 1)

	// Teardown is forced, so the domain goes and the claim with it, and the spec can then go too.
	ready, err := s.State().Teardown(s.Ctx(), spec.Metadata())
	s.Require().NoError(err)
	s.Require().False(ready, "the finalizer must prevent immediate deletion")

	s.assertFinalizer("recreated", false)
	s.Destroy(spec)

	s.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "recreated")

		return err != nil
	}, 5*time.Second, 10*time.Millisecond)

	recreated := s.startGracefully("recreated")
	s.drivePowerState(recreated, "stopped", hypervisorhelpers.StopModeGraceful.String())
	s.assertAsked("recreated", 2)
}

// An ask libvirt refused is one nothing was asked by, so it is asked again.
func (s *VirtualMachineDomainSuite) TestARefusedAskIsRetried() {
	s.start()

	spec := s.startGracefully("retry")

	s.client.mu.Lock()
	s.client.shutdownErr = errors.New("daemon is busy")
	s.client.mu.Unlock()

	s.drivePowerState(spec, "stopped", hypervisorhelpers.StopModeGraceful.String())

	// The attempt is observed even though it failed, and the claim is kept through it.
	select {
	case <-s.client.attemptedShutdown:
	case <-time.After(5 * time.Second):
		s.Require().Fail("no shutdown was attempted")
	}

	s.assertFinalizer("retry", true)
	s.Require().Zero(s.client.shutdownCount("retry"))

	s.client.mu.Lock()
	s.client.shutdownErr = nil
	s.client.mu.Unlock()

	s.assertAsked("retry", 1)
}
