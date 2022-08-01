// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/etcd"
)

func TestConfigSuite(t *testing.T) {
	suite.Run(t, &ConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&etcdctrl.ConfigController{}))
			},
		},
	})
}

type ConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ConfigSuite) TestReconcile() {
	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			EtcdConfig: &v1alpha1.EtcdConfig{
				ContainerImage: "foo/bar:v1.0.0",
				EtcdExtraArgs: map[string]string{
					"arg": "value",
				},
				EtcdSubnet: "10.0.0.0/8",
			},
		},
	}

	machineConfig := config.NewMachineConfig(cfg)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineConfig))

	suite.AssertWithin(3*time.Second, 100*time.Millisecond, ctest.WrapRetry(func(assert *assert.Assertions, require *require.Assertions) {
		etcdConfig, err := safe.StateGet[*etcd.Config](suite.Ctx(), suite.State(), etcd.NewConfig(etcd.NamespaceName, etcd.ConfigID).Metadata())
		if err != nil {
			assert.NoError(err)

			return
		}

		assert.Equal("foo/bar:v1.0.0", etcdConfig.TypedSpec().Image)
		assert.Equal(map[string]string{"arg": "value"}, etcdConfig.TypedSpec().ExtraArgs)
		assert.Equal([]string{"10.0.0.0/8"}, etcdConfig.TypedSpec().ValidSubnets)
	}))
}
