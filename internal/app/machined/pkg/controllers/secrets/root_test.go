// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestRootSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RootSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(secretsctrl.NewRootEtcdController()))
				suite.Require().NoError(suite.Runtime().RegisterController(secretsctrl.NewRootKubernetesController()))
				suite.Require().NoError(suite.Runtime().RegisterController(secretsctrl.NewRootOSController()))
			},
		},
	})
}

type RootSuite struct {
	ctest.DefaultSuite
}

func (suite *RootSuite) genConfig(controlplane bool) talosconfig.Config {
	input, err := generate.NewInput("test-cluster", "http://localhost:6443", "")
	suite.Require().NoError(err)

	var cfg talosconfig.Provider

	if controlplane {
		cfg, err = input.Config(machine.TypeControlPlane)
	} else {
		cfg, err = input.Config(machine.TypeWorker)
	}

	suite.Require().NoError(err)

	machineCfg := config.NewMachineConfig(cfg)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineCfg))

	return cfg
}

func (suite *RootSuite) TestReconcileControlPlane() {
	cfg := suite.genConfig(true)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.EtcdRootID},
		func(res *secrets.EtcdRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().EtcdCA, cfg.Cluster().Etcd().CA())
		},
	)
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.KubernetesRootID},
		func(res *secrets.KubernetesRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().CA, cfg.Cluster().CA())
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.OSRootID},
		func(res *secrets.OSRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().CA, cfg.Machine().Security().CA())
		},
	)
}

func (suite *RootSuite) TestReconcileWorker() {
	cfg := suite.genConfig(false)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.OSRootID},
		func(res *secrets.OSRoot, asrt *assert.Assertions) {
			asrt.Equal(res.TypedSpec().CA, cfg.Machine().Security().CA())
		},
	)

	rtestutils.AssertNoResource[*secrets.Etcd](suite.Ctx(), suite.T(), suite.State(), secrets.EtcdRootID)
	rtestutils.AssertNoResource[*secrets.Kubernetes](suite.Ctx(), suite.T(), suite.State(), secrets.KubernetesRootID)
}
