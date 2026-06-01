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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HostDNSConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *HostDNSConfigSuite) TestNoConfig() {
	ctest.AssertResource(suite, network.HostDNSConfigID, func(r *network.HostDNSConfig, asrt *assert.Assertions) {
		asrt.False(r.TypedSpec().Enabled)
		asrt.Equal(
			[]netip.AddrPort{netip.MustParseAddrPort("127.0.0.53:53")},
			r.TypedSpec().ListenAddresses,
		)
		asrt.Equal(netip.Addr{}, r.TypedSpec().ServiceHostDNSAddress)
		asrt.False(r.TypedSpec().ResolveMemberNames)
	})
}

func (suite *HostDNSConfigSuite) TestLegacyConfigEnabled() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy config
							HostDNSConfigEnabled:      new(true),
							HostDNSResolveMemberNames: new(true),
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{URL: u},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						PodSubnet: []string{constants.DefaultIPv4PodNet},
					},
				},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, network.HostDNSConfigID, func(r *network.HostDNSConfig, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Enabled)
		asrt.Equal(
			[]netip.AddrPort{netip.MustParseAddrPort("127.0.0.53:53")},
			r.TypedSpec().ListenAddresses,
		)
		asrt.Equal(netip.Addr{}, r.TypedSpec().ServiceHostDNSAddress)
		asrt.True(r.TypedSpec().ResolveMemberNames)
	})

	ctest.AssertNoResource[*network.AddressSpec](
		suite,
		network.LayeredID(network.ConfigOperator, network.AddressID("lo", netip.MustParsePrefix(constants.HostDNSAddress+"/32"))),
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *HostDNSConfigSuite) TestLegacyConfigForwardKubeDNSIPv4() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy config
							HostDNSConfigEnabled:        new(true),
							HostDNSForwardKubeDNSToHost: new(true),
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{URL: u},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						PodSubnet: []string{constants.DefaultIPv4PodNet, constants.DefaultIPv6PodNet},
					},
				},
			},
		),
	)

	suite.Create(cfg)

	hostDNSAddr := netip.MustParseAddr(constants.HostDNSAddress)
	hostDNSAddrV6 := netip.MustParseAddr(constants.HostDNSAddressV6)

	ctest.AssertResource(suite, network.HostDNSConfigID, func(r *network.HostDNSConfig, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Enabled)
		asrt.Equal(
			[]netip.AddrPort{
				netip.MustParseAddrPort("127.0.0.53:53"),
				netip.AddrPortFrom(hostDNSAddr, 53),
				netip.AddrPortFrom(hostDNSAddrV6, 53),
			},
			r.TypedSpec().ListenAddresses,
		)
		asrt.Equal(hostDNSAddr, r.TypedSpec().ServiceHostDNSAddress)
		asrt.Equal(hostDNSAddrV6, r.TypedSpec().ServiceHostDNSAddressV6)
	})

	for _, addrPrefix := range []netip.Prefix{
		netip.PrefixFrom(hostDNSAddr, hostDNSAddr.BitLen()),
		netip.PrefixFrom(hostDNSAddrV6, hostDNSAddrV6.BitLen()),
	} {
		ctest.AssertResource(
			suite,
			network.LayeredID(network.ConfigOperator, network.AddressID("lo", addrPrefix)),
			func(r *network.AddressSpec, asrt *assert.Assertions) {
				spec := r.TypedSpec()

				if addrPrefix.Addr().Is6() {
					asrt.Equal(nethelpers.FamilyInet6, spec.Family)
					asrt.Equal(nethelpers.ScopeGlobal, spec.Scope)
				} else {
					asrt.Equal(nethelpers.FamilyInet4, spec.Family)
					asrt.Equal(nethelpers.ScopeHost, spec.Scope)
				}

				asrt.Equal(addrPrefix, spec.Address)
				asrt.Equal("lo", spec.LinkName)
				asrt.Equal(nethelpers.AddressFlags(nethelpers.AddressPermanent), spec.Flags)
				asrt.Equal(network.ConfigOperator, spec.ConfigLayer)
			},
			rtestutils.WithNamespace(network.ConfigNamespaceName),
		)
	}
}

func (suite *HostDNSConfigSuite) TestLegacyConfigForwardKubeDNSIPv6Only() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy config
							HostDNSConfigEnabled:        new(true),
							HostDNSForwardKubeDNSToHost: new(true),
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{URL: u},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						PodSubnet: []string{constants.DefaultIPv6PodNet},
					},
				},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, network.HostDNSConfigID, func(r *network.HostDNSConfig, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Enabled)
		asrt.Equal(
			[]netip.AddrPort{
				netip.MustParseAddrPort("127.0.0.53:53"),
				netip.MustParseAddrPort("[" + constants.HostDNSAddressV6 + "]:53"),
			},
			r.TypedSpec().ListenAddresses,
		)
		asrt.Equal(netip.Addr{}, r.TypedSpec().ServiceHostDNSAddress)
		asrt.Equal(netip.MustParseAddr(constants.HostDNSAddressV6), r.TypedSpec().ServiceHostDNSAddressV6)
	})

	ctest.AssertNoResource[*network.AddressSpec](
		suite,
		network.LayeredID(network.ConfigOperator, network.AddressID("lo", netip.MustParsePrefix(constants.HostDNSAddress+"/32"))),
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *HostDNSConfigSuite) TestResolverConfigDocument() {
	rc := networkcfg.NewResolverConfigV1Alpha1()
	rc.ResolverHostDNS = networkcfg.HostDNSConfig{
		HostDNSEnabled:              new(true),
		HostDNSForwardKubeDNSToHost: new(true),
		HostDNSResolveMemberNames:   new(true),
	}

	v1 := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
				PodSubnet: []string{constants.DefaultIPv4PodNet},
			},
		},
	}

	ctr, err := container.New(v1, rc)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	hostDNSAddr := netip.MustParseAddr(constants.HostDNSAddress)

	ctest.AssertResource(suite, network.HostDNSConfigID, func(r *network.HostDNSConfig, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Enabled)
		asrt.True(r.TypedSpec().ResolveMemberNames)
		asrt.Equal(
			[]netip.AddrPort{
				netip.MustParseAddrPort("127.0.0.53:53"),
				netip.AddrPortFrom(hostDNSAddr, 53),
			},
			r.TypedSpec().ListenAddresses,
		)
		asrt.Equal(hostDNSAddr, r.TypedSpec().ServiceHostDNSAddress)
	})

	addrPrefix := netip.PrefixFrom(hostDNSAddr, hostDNSAddr.BitLen())

	ctest.AssertResource(
		suite,
		network.LayeredID(network.ConfigOperator, network.AddressID("lo", addrPrefix)),
		func(r *network.AddressSpec, asrt *assert.Assertions) {
			asrt.Equal(addrPrefix, r.TypedSpec().Address)
			asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
			asrt.Equal("lo", r.TypedSpec().LinkName)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func TestHostDNSConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &HostDNSConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.HostDNSConfigController{}))
			},
		},
	})
}
