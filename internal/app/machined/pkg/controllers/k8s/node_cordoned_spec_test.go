// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type NodeCordonedSuite struct {
	ctest.DefaultSuite
}

func TestNodeCordonedSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &NodeCordonedSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&k8sctrl.NodeCordonedSpecController{}))
			},
		},
	})
}

func (suite *NodeCordonedSuite) updateMachineStage(stage runtime.MachineStage) {
	status, err := safe.StateGetByID[*runtime.MachineStatus](suite.Ctx(), suite.State(), runtime.MachineStatusID)
	if err != nil && !state.IsNotFoundError(err) {
		suite.Require().NoError(err)
	}

	if status == nil {
		status = runtime.NewMachineStatus()
		status.TypedSpec().Stage = stage

		suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
	} else {
		status.TypedSpec().Stage = stage
		suite.Require().NoError(suite.State().Update(suite.Ctx(), status))
	}
}

func (suite *NodeCordonedSuite) TestBootingRunning() {
	suite.updateMachineStage(runtime.MachineStageBooting)

	rtestutils.AssertNoResource[*k8s.NodeCordonedSpec](suite.Ctx(), suite.T(), suite.State(), k8s.NodeCordonedID)

	suite.updateMachineStage(runtime.MachineStageRunning)

	rtestutils.AssertNoResource[*k8s.NodeCordonedSpec](suite.Ctx(), suite.T(), suite.State(), k8s.NodeCordonedID)
}

func (suite *NodeCordonedSuite) TestResetting() {
	suite.updateMachineStage(runtime.MachineStageRunning)

	rtestutils.AssertNoResource[*k8s.NodeCordonedSpec](suite.Ctx(), suite.T(), suite.State(), k8s.NodeCordonedID)

	suite.updateMachineStage(runtime.MachineStageResetting)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{k8s.NodeCordonedID},
		func(*k8s.NodeCordonedSpec, *assert.Assertions) {})
}
