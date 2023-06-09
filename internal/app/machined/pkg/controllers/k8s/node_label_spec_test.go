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
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type NodeLabelsSuite struct {
	ctest.DefaultSuite
}

func TestNodeLabelsSuite(t *testing.T) {
	suite.Run(t, &NodeLabelsSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&k8sctrl.NodeLabelSpecController{}))
			},
		},
	})
}

func (suite *NodeLabelsSuite) updateMachineConfig(machineType machine.Type, labels map[string]string) {
	cfg, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.V1Alpha1ID)
	if err != nil && !state.IsNotFoundError(err) {
		suite.Require().NoError(err)
	}

	if cfg == nil {
		cfg = config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
			MachineConfig: &v1alpha1.MachineConfig{
				MachineType:       machineType.String(),
				MachineNodeLabels: labels,
			},
		}))

		suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))
	} else {
		cfg.Container().RawV1Alpha1().MachineConfig.MachineNodeLabels = labels
		cfg.Container().RawV1Alpha1().MachineConfig.MachineType = machineType.String()
		suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))
	}
}

func (suite *NodeLabelsSuite) TestAddLabel() {
	// given
	expectedLabel := "expectedLabel"
	expectedValue := "expectedValue"

	// when
	suite.updateMachineConfig(machine.TypeWorker, map[string]string{
		expectedLabel: expectedValue,
	})

	// then
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{expectedLabel},
		func(labelSpec *k8s.NodeLabelSpec, asrt *assert.Assertions) {
			asrt.Equal(expectedValue, labelSpec.TypedSpec().Value)
		})
	rtestutils.AssertNoResource[*k8s.NodeLabelSpec](suite.Ctx(), suite.T(), suite.State(), constants.LabelNodeRoleControlPlane)
}

func (suite *NodeLabelsSuite) TestChangeLabel() {
	// given
	expectedLabel := "someLabel"
	oldValue := "oldValue"
	expectedValue := "newValue"

	// when
	suite.updateMachineConfig(machine.TypeControlPlane, map[string]string{
		expectedLabel: oldValue,
	})

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{expectedLabel},
		func(labelSpec *k8s.NodeLabelSpec, asrt *assert.Assertions) {
			asrt.Equal(oldValue, labelSpec.TypedSpec().Value)
		})

	suite.updateMachineConfig(machine.TypeControlPlane, map[string]string{
		expectedLabel: expectedValue,
	})

	// then
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{expectedLabel},
		func(labelSpec *k8s.NodeLabelSpec, asrt *assert.Assertions) {
			asrt.Equal(expectedValue, labelSpec.TypedSpec().Value)
		})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{constants.LabelNodeRoleControlPlane},
		func(labelSpec *k8s.NodeLabelSpec, asrt *assert.Assertions) {
			asrt.Empty(labelSpec.TypedSpec().Value)
		})
}

func (suite *NodeLabelsSuite) TestDeleteLabel() {
	// given
	expectedLabel := "label"
	expectedValue := "labelValue"

	// when
	suite.updateMachineConfig(machine.TypeWorker, map[string]string{
		expectedLabel: expectedValue,
	})

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{expectedLabel},
		func(labelSpec *k8s.NodeLabelSpec, asrt *assert.Assertions) {
			asrt.Equal(expectedValue, labelSpec.TypedSpec().Value)
		})

	suite.updateMachineConfig(machine.TypeWorker, map[string]string{})

	// then
	rtestutils.AssertNoResource[*k8s.NodeLabelSpec](suite.Ctx(), suite.T(), suite.State(), expectedLabel)
	rtestutils.AssertNoResource[*k8s.NodeLabelSpec](suite.Ctx(), suite.T(), suite.State(), constants.LabelNodeRoleControlPlane)
}
