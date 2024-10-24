// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"net"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	kubespanadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/kubespan"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	kubespanctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ManagerSuite struct {
	ctest.DefaultSuite

	mockWireguard *mockWireguardClient
}

func (suite *ManagerSuite) TestDisabled() {
	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = false

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertNoResource[*network.NfTablesChain](suite, "kubespan_outgoing")
}

type mockWireguardClient struct {
	deviceStateMu sync.Mutex
	deviceState   *wgtypes.Device
}

func (mock *mockWireguardClient) update(newState *wgtypes.Device) {
	mock.deviceStateMu.Lock()
	defer mock.deviceStateMu.Unlock()

	mock.deviceState = newState
}

func (mock *mockWireguardClient) Device(name string) (*wgtypes.Device, error) {
	mock.deviceStateMu.Lock()
	defer mock.deviceStateMu.Unlock()

	if mock.deviceState != nil {
		return mock.deviceState, nil
	}

	return nil, os.ErrNotExist
}

func (mock *mockWireguardClient) Close() error {
	return nil
}

type mockRulesManager struct{}

func (mock mockRulesManager) Install() error {
	return nil
}

func (mock mockRulesManager) Cleanup() error {
	return nil
}

func (suite *ManagerSuite) TestReconcile() {
	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().SharedSecret = "TPbGXrYlvuXgAl8dERpwjlA5tnEMoihPDPxlovcLtVg="
	cfg.TypedSpec().ForceRouting = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	mac, err := net.ParseMAC("ea:71:1b:b2:cc:ee")
	suite.Require().NoError(err)

	localIdentity := kubespan.NewIdentity(kubespan.NamespaceName, kubespan.LocalIdentity)
	suite.Require().NoError(kubespanadapter.IdentitySpec(localIdentity.TypedSpec()).GenerateKey())
	suite.Require().NoError(
		kubespanadapter.IdentitySpec(localIdentity.TypedSpec()).UpdateAddress(
			"v16UCWpO2iOm82n6F8dGCJ41ZXXBvDrjRDs2su7C_zs=",
			mac,
		),
	)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), localIdentity))

	// initial setup: link should be created without any peers
	ctest.AssertResource(suite,
		network.LayeredID(network.ConfigOperator, network.LinkID(constants.KubeSpanLinkName)),
		func(res *network.LinkSpec, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(network.ConfigOperator, spec.ConfigLayer)
			asrt.Equal(constants.KubeSpanLinkName, spec.Name)
			asrt.Equal(nethelpers.LinkNone, spec.Type)
			asrt.Equal("wireguard", spec.Kind)
			asrt.True(spec.Up)
			asrt.True(spec.Logical)

			asrt.Equal(localIdentity.TypedSpec().PrivateKey, spec.Wireguard.PrivateKey)
			asrt.Equal(constants.KubeSpanDefaultPort, spec.Wireguard.ListenPort)
			asrt.Equal(constants.KubeSpanDefaultFirewallMark, spec.Wireguard.FirewallMark)
			asrt.Len(spec.Wireguard.Peers, 0)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.AssertResource(suite,
		network.LayeredID(
			network.ConfigOperator,
			network.AddressID(constants.KubeSpanLinkName, localIdentity.TypedSpec().Address),
		), func(res *network.AddressSpec, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(localIdentity.TypedSpec().Address.Addr(), spec.Address.Addr())
			asrt.Equal(localIdentity.TypedSpec().Subnet.Bits(), spec.Address.Bits())
			asrt.Equal(network.ConfigOperator, spec.ConfigLayer)
			asrt.Equal(nethelpers.FamilyInet6, spec.Family)
			asrt.Equal(nethelpers.AddressFlags(nethelpers.AddressPermanent), spec.Flags)
			asrt.Equal(constants.KubeSpanLinkName, spec.LinkName)
			asrt.Equal(nethelpers.ScopeGlobal, spec.Scope)
		}, rtestutils.WithNamespace(network.ConfigNamespaceName))

	ctest.AssertResource(suite,
		network.LayeredID(
			network.ConfigOperator,
			network.RouteID(
				constants.KubeSpanDefaultRoutingTable,
				nethelpers.FamilyInet4,
				netip.Prefix{},
				netip.Addr{},
				1,
				"kubespan",
			),
		),
		func(res *network.RouteSpec, asrt *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.AssertResource(suite,
		network.LayeredID(
			network.ConfigOperator,
			network.RouteID(
				constants.KubeSpanDefaultRoutingTable,
				nethelpers.FamilyInet6,
				netip.Prefix{},
				netip.Addr{},
				1,
				"kubespan",
			),
		),
		func(res *network.RouteSpec, asrt *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	// add two peers, they should be added to the wireguard link spec and should be tracked in peer statuses
	peer1 := kubespan.NewPeerSpec(kubespan.NamespaceName, "3FxU7UuwektMjbyuJBs7i1hDj2rQA6tHnbNB6WrQxww=")
	peer1.TypedSpec().Address = netip.MustParseAddr("fd8a:4396:731e:e702:145e:c4ff:fe41:1ef9")
	peer1.TypedSpec().Label = "worker-1"
	peer1.TypedSpec().AllowedIPs = []netip.Prefix{
		netip.MustParsePrefix("10.244.1.0/24"),
	}
	peer1.TypedSpec().Endpoints = []netip.AddrPort{
		netip.MustParseAddrPort("172.20.0.3:51280"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), peer1))

	key1, err := wgtypes.ParseKey(peer1.Metadata().ID())
	suite.Require().NoError(err)

	peer2 := kubespan.NewPeerSpec(kubespan.NamespaceName, "tQuicRD0tqCu48M+zrySTe4slT15JxWhWIboZOB4tWs=")
	peer2.TypedSpec().Address = netip.MustParseAddr("fd8a:4396:731e:e702:9c83:cbff:fed0:f94b")
	peer2.TypedSpec().Label = "worker-2"
	peer2.TypedSpec().AllowedIPs = []netip.Prefix{
		netip.MustParsePrefix("10.244.2.0/24"),
	}
	peer2.TypedSpec().Endpoints = []netip.AddrPort{
		netip.MustParseAddrPort("172.20.0.4:51280"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), peer2))

	key2, err := wgtypes.ParseKey(peer2.Metadata().ID())
	suite.Require().NoError(err)

	ctest.AssertResource(suite,
		network.LayeredID(network.ConfigOperator, network.LinkID(constants.KubeSpanLinkName)),
		func(res *network.LinkSpec, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Len(spec.Wireguard.Peers, 2)

			if len(spec.Wireguard.Peers) != 2 {
				return
			}

			for i, peer := range []*kubespan.PeerSpec{peer1, peer2} {
				asrt.Equal(peer.Metadata().ID(), spec.Wireguard.Peers[i].PublicKey)
				asrt.Equal(cfg.TypedSpec().SharedSecret, spec.Wireguard.Peers[i].PresharedKey)
				asrt.Equal(peer.TypedSpec().AllowedIPs, spec.Wireguard.Peers[i].AllowedIPs)
				asrt.Equal(peer.TypedSpec().Endpoints[0].String(), spec.Wireguard.Peers[i].Endpoint)
			}
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	for _, peer := range []*kubespan.PeerSpec{peer1, peer2} {
		ctest.AssertResource(suite,
			peer.Metadata().ID(),
			func(res *kubespan.PeerStatus, asrt *assert.Assertions) {
				spec := res.TypedSpec()

				asrt.Equal(peer.TypedSpec().Label, spec.Label)
				asrt.Equal(kubespan.PeerStateUnknown, spec.State)
				asrt.Equal(peer.TypedSpec().Endpoints[0], spec.Endpoint)
				asrt.Equal(peer.TypedSpec().Endpoints[0], spec.LastUsedEndpoint)
				asrt.WithinDuration(time.Now(), spec.LastEndpointChange, 3*time.Second)
			},
		)
	}

	// check firewall rules
	ctest.AssertResource(suite,
		"kubespan_prerouting",
		func(res *network.NfTablesChain, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(nethelpers.ChainTypeFilter, spec.Type)
			asrt.Equal(nethelpers.ChainHookPrerouting, spec.Hook)
			asrt.Equal(nethelpers.ChainPriorityFilter, spec.Priority)
			asrt.Equal(nethelpers.VerdictAccept, spec.Policy)

			asrt.Len(spec.Rules, 2)

			if len(spec.Rules) != 2 {
				return
			}

			asrt.Equal(
				network.NfTablesRule{
					MatchMark: &network.NfTablesMark{
						Mask:  constants.KubeSpanDefaultFirewallMask,
						Value: constants.KubeSpanDefaultFirewallMark,
					},
					Verdict: pointer.To(nethelpers.VerdictAccept),
				},
				spec.Rules[0],
			)

			asrt.Equal(
				network.NfTablesRule{
					MatchDestinationAddress: &network.NfTablesAddressMatch{
						IncludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("10.244.1.0/24"),
							netip.MustParsePrefix("10.244.2.0/24"),
						},
					},
					SetMark: &network.NfTablesMark{
						Mask: ^uint32(constants.KubeSpanDefaultFirewallMask),
						Xor:  constants.KubeSpanDefaultForceFirewallMark,
					},
					Verdict: pointer.To(nethelpers.VerdictAccept),
				},
				spec.Rules[1],
			)
		},
	)

	// update config and disable force routing, nothing should be routed
	cfg.TypedSpec().ForceRouting = false
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	ctest.AssertResource(suite,
		"kubespan_prerouting",
		func(res *network.NfTablesChain, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(
				network.NfTablesRule{
					MatchDestinationAddress: &network.NfTablesAddressMatch{
						IncludeSubnets: []netip.Prefix{},
					},
					SetMark: &network.NfTablesMark{
						Mask: ^uint32(constants.KubeSpanDefaultFirewallMask),
						Xor:  constants.KubeSpanDefaultForceFirewallMark,
					},
					Verdict: pointer.To(nethelpers.VerdictAccept),
				},
				spec.Rules[1],
			)
		},
	)

	// report up status via wireguard mock
	suite.mockWireguard.update(
		&wgtypes.Device{
			Peers: []wgtypes.Peer{
				{
					PublicKey:         key1,
					Endpoint:          asUDP(peer1.TypedSpec().Endpoints[0]),
					LastHandshakeTime: time.Now(),
				},
				{
					PublicKey:         key2,
					Endpoint:          asUDP(peer2.TypedSpec().Endpoints[0]),
					LastHandshakeTime: time.Now(),
				},
			},
		},
	)

	for _, peer := range []*kubespan.PeerSpec{peer1, peer2} {
		ctest.AssertResource(suite,
			peer.Metadata().ID(),
			func(res *kubespan.PeerStatus, asrt *assert.Assertions) {
				spec := res.TypedSpec()

				asrt.Equal(kubespan.PeerStateUp, spec.State)
			},
		)
	}

	ctest.AssertResource(suite,
		"kubespan_prerouting",
		func(res *network.NfTablesChain, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(
				network.NfTablesRule{
					MatchDestinationAddress: &network.NfTablesAddressMatch{
						IncludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("10.244.1.0/24"),
							netip.MustParsePrefix("10.244.2.0/24"),
						},
					},
					SetMark: &network.NfTablesMark{
						Mask: ^uint32(constants.KubeSpanDefaultFirewallMask),
						Xor:  constants.KubeSpanDefaultForceFirewallMark,
					},
					Verdict: pointer.To(nethelpers.VerdictAccept),
				},
				spec.Rules[1],
			)
		},
	)

	// update config and disable wireguard, everything should be cleaned up
	cfg.TypedSpec().Enabled = false
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	ctest.AssertNoResource[*network.LinkSpec](
		suite,
		network.LayeredID(network.ConfigOperator, network.LinkID(constants.KubeSpanLinkName)),
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
	ctest.AssertNoResource[*network.NfTablesChain](
		suite,
		"kubespan_prerouting",
	)
}

func asUDP(addr netip.AddrPort) *net.UDPAddr {
	return &net.UDPAddr{
		IP:   addr.Addr().AsSlice(),
		Port: int(addr.Port()),
		Zone: addr.Addr().Zone(),
	}
}

func TestManagerSuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	mockWireguard := &mockWireguardClient{}

	suite.Run(t, &ManagerSuite{
		mockWireguard: mockWireguard,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&kubespanctrl.ManagerController{
					WireguardClientFactory: func() (kubespanctrl.WireguardClient, error) {
						return mockWireguard, nil
					},
					RulesManagerFactory: func(_ uint8, _, _ uint32) kubespanctrl.RulesManager {
						return mockRulesManager{}
					},
					PeerReconcileInterval: time.Second,
				}))
			},
		},
	})
}
