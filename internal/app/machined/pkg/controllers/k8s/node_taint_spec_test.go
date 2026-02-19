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
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type NodeTaintsSuite struct {
	ctest.DefaultSuite
}

func TestNodeTaintsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &NodeTaintsSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&k8sctrl.NodeTaintSpecController{}))
			},
		},
	})
}

func (suite *NodeTaintsSuite) updateMachineConfig(machineType machine.Type, allowScheduling bool, taints ...customTaint) {
	cfg, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		suite.Require().NoError(err)
	}

	nodeTaints := xslices.ToMap(taints, func(t customTaint) (string, string) { return t.key, t.value })

	if cfg == nil {
		cfg = config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
			MachineConfig: &v1alpha1.MachineConfig{
				MachineType:       machineType.String(),
				MachineNodeTaints: nodeTaints,
			},
			ClusterConfig: &v1alpha1.ClusterConfig{
				AllowSchedulingOnControlPlanes: new(allowScheduling),
			},
		}))

		suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))
	} else {
		cfg.Container().RawV1Alpha1().ClusterConfig.AllowSchedulingOnControlPlanes = new(allowScheduling)
		cfg.Container().RawV1Alpha1().MachineConfig.MachineType = machineType.String()
		cfg.Container().RawV1Alpha1().MachineConfig.MachineNodeTaints = nodeTaints
		suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))
	}
}

func (suite *NodeTaintsSuite) TestWorker() {
	suite.updateMachineConfig(machine.TypeWorker, false)

	rtestutils.AssertNoResource[*k8s.NodeTaintSpec](suite.Ctx(), suite.T(), suite.State(), constants.LabelNodeRoleControlPlane)
}

func (suite *NodeTaintsSuite) TestControlplane() {
	suite.updateMachineConfig(machine.TypeControlPlane, false)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{constants.LabelNodeRoleControlPlane},
		func(labelSpec *k8s.NodeTaintSpec, asrt *assert.Assertions) {
			asrt.Empty(labelSpec.TypedSpec().Value)
			asrt.Equal(string(v1.TaintEffectNoSchedule), labelSpec.TypedSpec().Effect)
		})

	suite.updateMachineConfig(machine.TypeControlPlane, true)

	rtestutils.AssertNoResource[*k8s.NodeTaintSpec](suite.Ctx(), suite.T(), suite.State(), constants.LabelNodeRoleControlPlane)
}

func (suite *NodeTaintsSuite) TestCustomTaints() {
	const customTaintKey = "key1"

	suite.updateMachineConfig(machine.TypeControlPlane, false, customTaint{
		key:   customTaintKey,
		value: "value1:NoSchedule",
	})

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{customTaintKey},
		func(labelSpec *k8s.NodeTaintSpec, asrt *assert.Assertions) {
			asrt.Equal(customTaintKey, labelSpec.TypedSpec().Key)
			asrt.Equal("value1", labelSpec.TypedSpec().Value)
			asrt.Equal(string(v1.TaintEffectNoSchedule), labelSpec.TypedSpec().Effect)
		})

	suite.updateMachineConfig(machine.TypeControlPlane, false)

	rtestutils.AssertNoResource[*k8s.NodeTaintSpec](suite.Ctx(), suite.T(), suite.State(), customTaintKey)
}

type customTaint struct {
	key   string
	value string
}
