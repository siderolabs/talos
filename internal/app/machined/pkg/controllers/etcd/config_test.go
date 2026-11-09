// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	etcdcfg "github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

func TestConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ConfigSuite{
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(etcdctrl.NewConfigController()))
		},
	})
}

type ConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ConfigSuite) TestReconcile() {
	for _, tt := range []struct {
		name           string
		etcdConfig     *v1alpha1.EtcdConfig //nolint:staticcheck // testing legacy config
		networkConfig  v1alpha1.NetworkDeviceList
		expectedConfig etcd.ConfigSpec
		extraConfig    func(*testing.T) []configconfig.Document
	}{
		{
			name: "default config",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
				ContainerImage: "foo/bar:v1.0.0",
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                 "foo/bar:v1.0.0",
				ExtraArgs:             map[string]etcd.ArgValues{},
				AdvertiseValidSubnets: nil,
				ListenValidSubnets:    nil,
			},
		},
		{
			name: "extra args config",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
				ContainerImage: "foo/bar:v1.0.0",
				EtcdExtraArgs: meta.Args{
					"foo": meta.NewArgValue("", []string{"bar", "baz"}),
				},
			},
			expectedConfig: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				ExtraArgs: map[string]etcd.ArgValues{
					"foo": {Values: []string{"bar", "baz"}},
				},
				AdvertiseValidSubnets: nil,
				ListenValidSubnets:    nil,
			},
		},
		{
			name: "legacy subnet",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
				ContainerImage: "foo/bar:v1.0.0",
				EtcdExtraArgs: meta.Args{
					"arg": meta.NewArgValue("value", nil),
				},
				EtcdSubnet: "10.0.0.0/8", //nolint:staticcheck // testing deprecated field
			},
			expectedConfig: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				ExtraArgs: map[string]etcd.ArgValues{
					"arg": {Values: []string{"value"}},
				},
				AdvertiseValidSubnets: []string{"10.0.0.0/8"},
				ListenValidSubnets:    nil,
			},
		},
		{
			name: "advertised subnets",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
				ContainerImage:        "foo/bar:v1.0.0",
				EtcdAdvertisedSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                 "foo/bar:v1.0.0",
				ExtraArgs:             map[string]etcd.ArgValues{},
				AdvertiseValidSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
				ListenValidSubnets:    []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
		},
		{
			name: "advertised and listen subnets",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
				ContainerImage:        "foo/bar:v1.0.0",
				EtcdAdvertisedSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
				EtcdListenSubnets:     []string{"10.0.0.0/8"},
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                 "foo/bar:v1.0.0",
				ExtraArgs:             map[string]etcd.ArgValues{},
				AdvertiseValidSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
				ListenValidSubnets:    []string{"10.0.0.0/8"},
			},
		},
		{
			name: "default with legacy vip",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
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
				ExtraArgs:               map[string]etcd.ArgValues{},
				AdvertiseValidSubnets:   nil,
				AdvertiseExcludeSubnets: []string{"10.0.0.4"},
				ListenValidSubnets:      nil,
			},
		},
		{
			name: "default with new vip",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
				ContainerImage: "foo/bar:v1.0.0",
			},
			extraConfig: func(*testing.T) []configconfig.Document {
				vipCfg := network.NewLayer2VIPConfigV1Alpha1("10.0.0.4")
				vipCfg.LinkName = "eth0"

				return []configconfig.Document{vipCfg}
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                   "foo/bar:v1.0.0",
				ExtraArgs:               map[string]etcd.ArgValues{},
				AdvertiseValidSubnets:   nil,
				AdvertiseExcludeSubnets: []string{"10.0.0.4"},
				ListenValidSubnets:      nil,
			},
		},
		{
			name: "advertised with legacy vip",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
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
				ExtraArgs:               map[string]etcd.ArgValues{},
				AdvertiseValidSubnets:   []string{"10.0.0.0/8", "192.168.0.0/24"},
				AdvertiseExcludeSubnets: []string{"10.0.0.4"},
				ListenValidSubnets:      []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
		},
		{
			name: "advertised with new vip",
			etcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing legacy config
				ContainerImage:        "foo/bar:v1.0.0",
				EtcdAdvertisedSubnets: []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
			extraConfig: func(*testing.T) []configconfig.Document {
				vipCfg := network.NewLayer2VIPConfigV1Alpha1("10.0.0.4")
				vipCfg.LinkName = "eth0"

				return []configconfig.Document{vipCfg}
			},
			expectedConfig: etcd.ConfigSpec{
				Image:                   "foo/bar:v1.0.0",
				ExtraArgs:               map[string]etcd.ArgValues{},
				AdvertiseValidSubnets:   []string{"10.0.0.0/8", "192.168.0.0/24"},
				AdvertiseExcludeSubnets: []string{"10.0.0.4"},
				ListenValidSubnets:      []string{"10.0.0.0/8", "192.168.0.0/24"},
			},
		},
	} {
		suite.Run(tt.name, func() {
			// legacy etcd config is in effect only if the CA is set
			tt.etcdConfig.RootCA = &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
				Key: []byte("key"),
			}

			cfgV1Alpha1 := &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					EtcdConfig: tt.etcdConfig, //nolint:staticcheck // testing legacy config
				},
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkInterfaces: tt.networkConfig, //nolint:staticcheck // legacy config
					},
				},
			}

			documents := []configconfig.Document{cfgV1Alpha1}

			if tt.extraConfig != nil {
				documents = slices.Concat(documents, tt.extraConfig(suite.T()))
			}

			ctr, err := container.New(documents...)
			suite.Require().NoError(err)

			machineConfig := config.NewMachineConfig(ctr)
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

func (suite *ConfigSuite) TestReconcileMultiDoc() {
	memberCfg := etcdcfg.NewEtcdMemberConfigV1Alpha1()
	memberCfg.MemberExtraArgs = meta.Args{
		"arg": meta.NewArgValue("value", nil),
	}
	memberCfg.MemberAdvertisedSubnets = []string{"10.0.0.0/8", "!10.0.0.3/32"}
	memberCfg.MemberListenSubnets = []string{"10.0.0.0/8"}

	ctr, err := container.New(
		&v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{},
			MachineConfig: &v1alpha1.MachineConfig{
				MachineType: "controlplane",
			},
		},
		memberCfg,
	)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(ctr)))

	ctest.AssertResource(suite, etcd.ConfigID, func(etcdConfig *etcd.Config, asrt *assert.Assertions) {
		asrt.Equal(etcd.ConfigSpec{
			Image: memberCfg.Image(),
			ExtraArgs: map[string]etcd.ArgValues{
				"arg": {Values: []string{"value"}},
			},
			AdvertiseValidSubnets: []string{"10.0.0.0/8", "!10.0.0.3/32"},
			ListenValidSubnets:    []string{"10.0.0.0/8"},
		}, *etcdConfig.TypedSpec())
	})
}

func (suite *ConfigSuite) TestReconcileNoMember() {
	ctr, err := container.New(
		&v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{},
			MachineConfig: &v1alpha1.MachineConfig{
				MachineType: "controlplane",
			},
		},
	)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(ctr)))

	ctest.AssertNoResource[*etcd.Config](suite, etcd.ConfigID)
}
