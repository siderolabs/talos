// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// ghost is a record of a stop of a machine which is declared nowhere and has no domain, so it is
// always retired. Cases record it after their other inputs and assert it is gone before asserting
// what survived, which is what makes "the record is still there" mean the controller looked at it
// rather than has not run yet.
const ghost = "ghost"

type VirtualMachineStopModeSuite struct {
	ctest.DefaultSuite
}

func TestVirtualMachineStopModeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VirtualMachineStopModeSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineStopModeController{}))
		},
	})
}

// record writes a graceful stop the way the power API does, stamped with the owner which lets the
// controller destroy it.
func (suite *VirtualMachineStopModeSuite) record(name string) {
	suite.T().Helper()

	mode := hypervisor.NewVirtualMachineStopMode(hypervisor.NamespaceName, name)
	mode.TypedSpec().Mode = hypervisorhelpers.StopModeGraceful.String()

	suite.Require().NoError(suite.State().Create(suite.Ctx(), mode,
		state.WithCreateOwner(hypervisor.VirtualMachineStopModeOwner)))
}

func (suite *VirtualMachineStopModeSuite) declare(powerState hypervisorhelpers.PowerState) {
	suite.T().Helper()

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	spec.TypedSpec().PowerState = powerState.String()

	suite.Create(spec)
}

// start drives vmName towards running, the way any path which starts it does.
func (suite *VirtualMachineStopModeSuite) start() {
	suite.T().Helper()

	_, err := safe.StateUpdateWithConflicts(suite.Ctx(), suite.State(),
		hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName).Metadata(),
		func(spec *hypervisor.VirtualMachineSpec) error {
			spec.TypedSpec().PowerState = hypervisorhelpers.PowerStateRunning.String()

			return nil
		})
	suite.Require().NoError(err)
}

// define publishes the domain spec of vmName, claimed by VirtualMachineController when it may still have a
// domain to stop.
func (suite *VirtualMachineStopModeSuite) define(claimed bool) {
	suite.T().Helper()

	domainSpec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, vmName)
	suite.Create(domainSpec)

	if claimed {
		suite.AddFinalizer(domainSpec.Metadata(), (&hypervisorctrl.VirtualMachineController{}).Name())
	}
}

// settle records a ghost after every other input and waits for it to be retired, so that the
// controller is known to have looked at everything written before.
func (suite *VirtualMachineStopModeSuite) settle() {
	suite.T().Helper()

	suite.record(ghost)
	ctest.AssertNoResource[*hypervisor.VirtualMachineStopMode](suite, ghost)
}

func (suite *VirtualMachineStopModeSuite) assertRecorded(name string) {
	suite.T().Helper()

	suite.settle()
	ctest.AssertResource(suite, name, func(mode *hypervisor.VirtualMachineStopMode, asrt *assert.Assertions) {
		asrt.Equal(hypervisorhelpers.StopModeGraceful.String(), mode.TypedSpec().Mode)
	})
}

// A stop in flight is one whose domain VirtualMachineController still holds. No domain status is
// published here: its absence is not evidence the domain is gone.
func (suite *VirtualMachineStopModeSuite) TestKeepsAStopWhichHasADomainToStop() {
	suite.record(vmName)
	suite.declare(hypervisorhelpers.PowerStateStopped)
	suite.define(true)

	suite.assertRecorded(vmName)
}

// A machine driven towards running is one whose stop has been recorded but not yet driven: the
// power API writes the record before it patches the machine configuration.
func (suite *VirtualMachineStopModeSuite) TestKeepsAStopWhichHasNotBeenDrivenYet() {
	suite.record(vmName)
	suite.declare(hypervisorhelpers.PowerStateRunning)
	suite.define(true)

	suite.assertRecorded(vmName)
}

// No domain held and nothing driving towards one: the stop is over.
func (suite *VirtualMachineStopModeSuite) TestRetiresAStopWhichIsOver() {
	suite.record(vmName)
	suite.declare(hypervisorhelpers.PowerStateStopped)
	suite.define(false)

	ctest.AssertNoResource[*hypervisor.VirtualMachineStopMode](suite, vmName)
}

// A stop seen driven and then overtaken by a start, by whatever path, is abandoned: it must not
// decide how a later stop is carried out.
func (suite *VirtualMachineStopModeSuite) TestRetiresAStopOvertakenByAStart() {
	suite.record(vmName)
	suite.declare(hypervisorhelpers.PowerStateStopped)
	suite.define(true)

	suite.assertRecorded(vmName)

	suite.start()

	ctest.AssertNoResource[*hypervisor.VirtualMachineStopMode](suite, vmName)
}

// A machine which left the machine configuration takes the record of its stop with it.
func (suite *VirtualMachineStopModeSuite) TestRetiresAStopOfAMachineWhichIsGone() {
	suite.record(vmName)

	ctest.AssertNoResource[*hypervisor.VirtualMachineStopMode](suite, vmName)
}
