// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type OperatorVIPConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *OperatorVIPConfigSuite) assertOperators(
	requiredIDs []string,
	check func(*network.OperatorSpec, *assert.Assertions),
) {
	ctest.AssertResources(suite, requiredIDs, check, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *OperatorVIPConfigSuite) TestMachineConfigurationLegacyVIP() {
	for _, link := range []struct {
		name    string
		aliases []string
	}{
		{
			name:    "eth5",
			aliases: []string{"enxa"},
		},
		{
			name:    "eth6",
			aliases: []string{"enxb"},
		},
	} {
		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().AltNames = link.aliases

		suite.Create(status)
	}

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth1",
								DeviceDHCP:      pointer.To(true),
								DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
									SharedIP: "2.3.4.5",
								},
							},
							{
								DeviceInterface: "eth2",
								DeviceDHCP:      pointer.To(true),
								DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
									SharedIP: "fd7a:115c:a1e0:ab12:4843:cd96:6277:2302",
								},
							},
							{
								DeviceInterface: "eth3",
								DeviceDHCP:      pointer.To(true),
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID: 26,
										VlanVIP: &v1alpha1.DeviceVIPConfig{
											SharedIP: "5.5.4.4",
										},
									},
								},
							},
							{
								DeviceInterface: "enxa",
								DeviceDHCP:      pointer.To(true),
								DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
									SharedIP: "2.3.4.5",
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.Create(cfg)

	suite.assertOperators(
		[]string{
			"configuration/vip/eth1/2.3.4.5",
			"configuration/vip/eth2/fd7a:115c:a1e0:ab12:4843:cd96:6277:2302",
			"configuration/vip/eth3.26/5.5.4.4",
			"configuration/vip/eth5/2.3.4.5",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorVIP, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/vip/eth1/2.3.4.5":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
				asrt.EqualValues(netip.MustParseAddr("2.3.4.5"), r.TypedSpec().VIP.IP)
			case "configuration/vip/eth5/2.3.4.5":
				asrt.Equal("eth5", r.TypedSpec().LinkName)
				asrt.EqualValues(netip.MustParseAddr("2.3.4.5"), r.TypedSpec().VIP.IP)
			case "configuration/vip/eth2/fd7a:115c:a1e0:ab12:4843:cd96:6277:2302":
				asrt.Equal("eth2", r.TypedSpec().LinkName)
				asrt.EqualValues(
					netip.MustParseAddr("fd7a:115c:a1e0:ab12:4843:cd96:6277:2302"),
					r.TypedSpec().VIP.IP,
				)
			case "configuration/vip/eth3.26/5.5.4.4":
				asrt.Equal("eth3.26", r.TypedSpec().LinkName)
				asrt.EqualValues(netip.MustParseAddr("5.5.4.4"), r.TypedSpec().VIP.IP)
			}
		},
	)
}

func (suite *OperatorVIPConfigSuite) TestMachineConfigurationVIP() {
	for _, link := range []struct {
		name    string
		aliases []string
	}{
		{
			name:    "eth5",
			aliases: []string{"enxa"},
		},
		{
			name:    "eth6",
			aliases: []string{"enxb"},
		},
	} {
		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().AltNames = link.aliases

		suite.Create(status)
	}

	vip1 := networkcfg.NewLayer2VIPConfigV1Alpha1("2.3.4.5")
	vip1.LinkName = "eth33"

	vip2 := networkcfg.NewLayer2VIPConfigV1Alpha1("fd7a:115c:a1e0:ab12:4843:cd96:6277:2302")
	vip2.LinkName = "enxa"

	ctr, err := container.New(vip1, vip2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.assertOperators(
		[]string{
			"configuration/vip/eth33/2.3.4.5",
			"configuration/vip/eth5/fd7a:115c:a1e0:ab12:4843:cd96:6277:2302",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorVIP, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/vip/eth33/2.3.4.5":
				asrt.Equal("eth33", r.TypedSpec().LinkName)
				asrt.EqualValues(netip.MustParseAddr("2.3.4.5"), r.TypedSpec().VIP.IP)
			case "configuration/vip/eth5/fd7a:115c:a1e0:ab12:4843:cd96:6277:2302":
				asrt.Equal("eth5", r.TypedSpec().LinkName)
				asrt.EqualValues(
					netip.MustParseAddr("fd7a:115c:a1e0:ab12:4843:cd96:6277:2302"),
					r.TypedSpec().VIP.IP,
				)
			}
		},
	)
}

func TestOperatorVIPConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &OperatorVIPConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.DeviceConfigController{}))
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.OperatorVIPConfigController{}))
			},
		},
	})
}
