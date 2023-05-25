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

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
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

	for _, tt := range []struct {
		name           string
		etcdConfig     *v1alpha1.EtcdConfig
		networkConfig  v1alpha1.NetworkDeviceList
		expectedConfig etcd.ConfigSpec
	}{
		{
			name: "default config",
			etcdConfig: &v1alpha1.EtcdConfig{
				ContainerImage: "foo/bar:v1.0.0",
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                 "foo/bar:v1.0.0",
				ExtraArgs:             map[string]string{},
				AdvertiseValidSubnets: nil,
				ListenValidSubnets:    nil,
			},
		},
		{
			name: "legacy subnet",
			etcdConfig: &v1alpha1.EtcdConfig{
				ContainerImage: "foo/bar:v1.0.0",
				EtcdExtraArgs: map[string]string{
					"arg": "value",
				},
				EtcdSubnet: "10.0.0.0/8",
			},
			expectedConfig: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				ExtraArgs: map[string]string{
					"arg": "value",
				},
				AdvertiseValidSubnets: []string{"10.0.0.0/8"},
				ListenValidSubnets:    nil,
			},
		},
		{
			name: "advertised subnets",
			etcdConfig: &v1alpha1.EtcdConfig{
				ContainerImage:        "foo/bar:v1.0.0",
				EtcdAdvertisedSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                 "foo/bar:v1.0.0",
				ExtraArgs:             map[string]string{},
				AdvertiseValidSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
				ListenValidSubnets:    []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
		},
		{
			name: "advertised and listen subnets",
			etcdConfig: &v1alpha1.EtcdConfig{
				ContainerImage:        "foo/bar:v1.0.0",
				EtcdAdvertisedSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
				EtcdListenSubnets:     []string{"10.0.0.0/8"},
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                 "foo/bar:v1.0.0",
				ExtraArgs:             map[string]string{},
				AdvertiseValidSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
				ListenValidSubnets:    []string{"10.0.0.0/8"},
			},
		},
		{
			name: "default with vip",
			etcdConfig: &v1alpha1.EtcdConfig{
				ContainerImage: "foo/bar:v1.0.0",
			},
			networkConfig: v1alpha1.NetworkDeviceList{
				{
					DeviceInterface: "eth0",
					DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
						SharedIP: "10.0.0.4",
					},
				},
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                   "foo/bar:v1.0.0",
				ExtraArgs:               map[string]string{},
				AdvertiseValidSubnets:   nil,
				AdvertiseExcludeSubnets: []string{"10.0.0.4"},
				ListenValidSubnets:      nil,
			},
		},
		{
			name: "advertised with vip",
			etcdConfig: &v1alpha1.EtcdConfig{
				ContainerImage:        "foo/bar:v1.0.0",
				EtcdAdvertisedSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
			networkConfig: v1alpha1.NetworkDeviceList{
				{
					DeviceInterface: "eth0",
					DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
						SharedIP: "10.0.0.4",
					},
				},
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                   "foo/bar:v1.0.0",
				ExtraArgs:               map[string]string{},
				AdvertiseValidSubnets:   []string{"10.0.0.0/8", "192.168.0.0/24"},
				AdvertiseExcludeSubnets: []string{"10.0.0.4"},
				ListenValidSubnets:      []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
		},
	} {
		suite.Run(tt.name, func() {
			cfg := container.NewV1Alpha1(&v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					EtcdConfig: tt.etcdConfig,
				},
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: tt.networkConfig,
					},
				},
			})

			machineConfig := config.NewMachineConfig(cfg)
			suite.Require().NoError(suite.State().Create(suite.Ctx(), machineConfig))

			suite.AssertWithin(3*time.Second, 100*time.Millisecond, ctest.WrapRetry(func(assert *assert.Assertions, require *require.Assertions) {
				etcdConfig, err := safe.StateGet[*etcd.Config](suite.Ctx(), suite.State(), etcd.NewConfig(etcd.NamespaceName, etcd.ConfigID).Metadata())
				if err != nil {
					assert.NoError(err)

					return
				}

				assert.Equal(tt.expectedConfig, *etcdConfig.TypedSpec())
			}))

			suite.Require().NoError(suite.State().Destroy(suite.Ctx(), machineConfig.Metadata()))
		})
	}
}
