// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// cpuPartitionReady publishes the status of an applied CPU partition policy, which containers wait for.
func cpuPartitionReady(s ctest.Suite) {
	status := runtime.NewCPUPartitionStatus()
	status.TypedSpec().Ready = true
	s.Require().NoError(s.State().Create(s.Ctx(), status))
}

func (suite *RuntimeSuite) setCPUPartition(ready bool) {
	ctest.UpdateWithConflicts(suite, runtime.NewCPUPartitionStatus(), func(res *runtime.CPUPartitionStatus) error {
		*res.TypedSpec() = runtime.CPUPartitionStatusSpec{Ready: ready}

		return nil
	})
}

func (suite *RuntimeSuite) assertWaiting(id string) {
	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseCreated, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, "waiting to start")
	})

	runner := suite.runner

	suite.Require().Never(func() bool { return runner.wasStarted(id) }, 200*time.Millisecond, 10*time.Millisecond)
}

func (suite *RuntimeSuite) assertStarted(id string) {
	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
		asrt.Empty(status.TypedSpec().Error)
	})
}

func (suite *RuntimeSuite) TestCPUPartitionGatesFirstStart() {
	markCRIUp(suite)
	suite.createLifecycle()
	suite.setCPUPartition(false)

	const id = "pinned-0"

	suite.createInstanceSpec(id)
	suite.assertWaiting(id)

	suite.setCPUPartition(true)
	suite.assertStarted(id)
}

func (suite *RuntimeSuite) TestNoCPUPartitionStatusGatesStart() {
	markCRIUp(suite)
	suite.createLifecycle()
	suite.Destroy(runtime.NewCPUPartitionStatus())

	const id = "pinned-1"

	suite.createInstanceSpec(id)
	suite.assertWaiting(id)
}

// An explicit stop still reaches a running container whatever the CPU partition status says.
func (suite *RuntimeSuite) TestExplicitStopIgnoresCPUPartition() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "pinned-2"

	suite.createInstanceSpec(id)
	suite.assertStarted(id)

	suite.setCPUPartition(false)

	md := containers.NewContainerInstanceSpec(containers.NamespaceName, id).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), md)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), md, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Assert().True(suite.runner.wasRemoved(id))
	suite.Assert().Equal(1, suite.runner.startCount(id))
}
