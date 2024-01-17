// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go4.org/netipx"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	kubespanadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/kubespan"
	kubespanctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ManagerSuite struct {
	KubeSpanSuite
}

func (suite *ManagerSuite) TestDisabled() {
	suite.Require().NoError(suite.runtime.RegisterController(&kubespanctrl.ManagerController{}))

	suite.startRuntime()

	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = false

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResourceType(
				resource.NewMetadata(kubespan.NamespaceName, kubespan.PeerStatusType, "", resource.VersionUndefined),
			),
		),
	)
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

type mockNftablesManager struct {
	mu    sync.Mutex
	ipSet *netipx.IPSet
}

func (mock *mockNftablesManager) Update(ipSet *netipx.IPSet, mtu uint32) error {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	mock.ipSet = ipSet

	return nil
}

func (mock *mockNftablesManager) Cleanup() error {
	return nil
}

func (mock *mockNftablesManager) IPSet() *netipx.IPSet {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.ipSet
}

//nolint:gocyclo
func (suite *ManagerSuite) TestReconcile() {
	mockWireguard := &mockWireguardClient{}
	mockNfTables := &mockNftablesManager{}

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&kubespanctrl.ManagerController{
				WireguardClientFactory: func() (kubespanctrl.WireguardClient, error) {
					return mockWireguard, nil
				},
				RulesManagerFactory: func(_, _, _ int) kubespanctrl.RulesManager {
					return mockRulesManager{}
				},
				NfTablesManagerFactory: func(_, _, _ uint32) kubespanctrl.NfTablesManager {
					return mockNfTables
				},
				PeerReconcileInterval: time.Second,
			},
		),
	)

	suite.startRuntime()

	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().SharedSecret = "TPbGXrYlvuXgAl8dERpwjlA5tnEMoihPDPxlovcLtVg="
	cfg.TypedSpec().ForceRouting = true
	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

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
	suite.Require().NoError(suite.state.Create(suite.ctx, localIdentity))

	// initial setup: link should be created without any peers
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(
				resource.NewMetadata(
					network.ConfigNamespaceName,
					network.LinkSpecType,
					network.LayeredID(network.ConfigOperator, network.LinkID(constants.KubeSpanLinkName)),
					resource.VersionUndefined,
				),
				func(res resource.Resource) error {
					spec := res.(*network.LinkSpec).TypedSpec()

					suite.Assert().Equal(network.ConfigOperator, spec.ConfigLayer)
					suite.Assert().Equal(constants.KubeSpanLinkName, spec.Name)
					suite.Assert().Equal(nethelpers.LinkNone, spec.Type)
					suite.Assert().Equal("wireguard", spec.Kind)
					suite.Assert().True(spec.Up)
					suite.Assert().True(spec.Logical)

					suite.Assert().Equal(localIdentity.TypedSpec().PrivateKey, spec.Wireguard.PrivateKey)
					suite.Assert().Equal(constants.KubeSpanDefaultPort, spec.Wireguard.ListenPort)
					suite.Assert().Equal(constants.KubeSpanDefaultFirewallMark, spec.Wireguard.FirewallMark)
					suite.Assert().Len(spec.Wireguard.Peers, 0)

					return nil
				},
			),
		),
	)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(
				resource.NewMetadata(
					network.ConfigNamespaceName,
					network.AddressSpecType,
					network.LayeredID(
						network.ConfigOperator,
						network.AddressID(constants.KubeSpanLinkName, localIdentity.TypedSpec().Address),
					),
					resource.VersionUndefined,
				),
				func(res resource.Resource) error {
					spec := res.(*network.AddressSpec).TypedSpec()

					suite.Assert().Equal(localIdentity.TypedSpec().Address.Addr(), spec.Address.Addr())
					suite.Assert().Equal(localIdentity.TypedSpec().Subnet.Bits(), spec.Address.Bits())
					suite.Assert().Equal(network.ConfigOperator, spec.ConfigLayer)
					suite.Assert().Equal(nethelpers.FamilyInet6, spec.Family)
					suite.Assert().Equal(nethelpers.AddressFlags(nethelpers.AddressPermanent), spec.Flags)
					suite.Assert().Equal(constants.KubeSpanLinkName, spec.LinkName)
					suite.Assert().Equal(nethelpers.ScopeGlobal, spec.Scope)

					return nil
				},
			),
		),
	)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResourceIDs(
				resource.NewMetadata(
					network.ConfigNamespaceName,
					network.RouteSpecType,
					"",
					resource.VersionUndefined,
				),
				[]resource.ID{
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
				},
			),
		),
	)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResourceType(
				resource.NewMetadata(kubespan.NamespaceName, kubespan.PeerStatusType, "", resource.VersionUndefined),
			),
		),
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
	suite.Require().NoError(suite.state.Create(suite.ctx, peer1))

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
	suite.Require().NoError(suite.state.Create(suite.ctx, peer2))

	key2, err := wgtypes.ParseKey(peer2.Metadata().ID())
	suite.Require().NoError(err)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(
				resource.NewMetadata(
					network.ConfigNamespaceName,
					network.LinkSpecType,
					network.LayeredID(network.ConfigOperator, network.LinkID(constants.KubeSpanLinkName)),
					resource.VersionUndefined,
				),
				func(res resource.Resource) error {
					spec := res.(*network.LinkSpec).TypedSpec()

					if len(spec.Wireguard.Peers) != 2 {
						return retry.ExpectedErrorf("peers not set up yet")
					}

					for i, peer := range []*kubespan.PeerSpec{peer1, peer2} {
						suite.Assert().Equal(peer.Metadata().ID(), spec.Wireguard.Peers[i].PublicKey)
						suite.Assert().Equal(cfg.TypedSpec().SharedSecret, spec.Wireguard.Peers[i].PresharedKey)
						suite.Assert().Equal(peer.TypedSpec().AllowedIPs, spec.Wireguard.Peers[i].AllowedIPs)
						suite.Assert().Equal(peer.TypedSpec().Endpoints[0].String(), spec.Wireguard.Peers[i].Endpoint)
					}

					return nil
				},
			),
		),
	)

	for _, peer := range []*kubespan.PeerSpec{peer1, peer2} {
		peer := peer

		suite.Assert().NoError(
			retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				suite.assertResource(
					resource.NewMetadata(
						kubespan.NamespaceName,
						kubespan.PeerStatusType,
						peer.Metadata().ID(),
						resource.VersionUndefined,
					),
					func(res resource.Resource) error {
						spec := res.(*kubespan.PeerStatus).TypedSpec()

						suite.Assert().Equal(peer.TypedSpec().Label, spec.Label)
						suite.Assert().Equal(kubespan.PeerStateUnknown, spec.State)
						suite.Assert().Equal(peer.TypedSpec().Endpoints[0], spec.Endpoint)
						suite.Assert().Equal(peer.TypedSpec().Endpoints[0], spec.LastUsedEndpoint)
						suite.Assert().WithinDuration(time.Now(), spec.LastEndpointChange, 3*time.Second)

						return nil
					},
				),
			),
		)
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				ipSet := mockNfTables.IPSet()

				if ipSet == nil {
					return retry.ExpectedErrorf("ipset is nil")
				}

				ranges := fmt.Sprintf("%v", ipSet.Ranges())
				expected := "[10.244.1.0-10.244.2.255]"

				if ranges != expected {
					return retry.ExpectedErrorf("ranges %s != expected %s", ranges, expected)
				}

				return nil
			},
		),
	)

	// update config and disable force routing, nothing should be routed
	cfg.TypedSpec().ForceRouting = false
	suite.Require().NoError(suite.state.Update(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				ipSet := mockNfTables.IPSet()

				if ipSet == nil {
					return retry.ExpectedErrorf("ipset is nil")
				}

				if len(ipSet.Prefixes()) != 0 {
					return retry.ExpectedErrorf("expected empty ipset: %v", ipSet.Ranges())
				}

				return nil
			},
		),
	)

	// report up status via wireguard mock
	mockWireguard.update(
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
		peer := peer

		suite.Assert().NoError(
			retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				suite.assertResource(
					resource.NewMetadata(
						kubespan.NamespaceName,
						kubespan.PeerStatusType,
						peer.Metadata().ID(),
						resource.VersionUndefined,
					),
					func(res resource.Resource) error {
						spec := res.(*kubespan.PeerStatus).TypedSpec()

						if spec.State != kubespan.PeerStateUp {
							return retry.ExpectedErrorf("peer state is not up yet: %s", spec.State)
						}

						return nil
					},
				),
			),
		)
	}

	// as the peers are now up, all traffic should be routed via KubeSpan
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				ipSet := mockNfTables.IPSet()

				if ipSet == nil {
					return retry.ExpectedErrorf("ipset is nil")
				}

				ranges := fmt.Sprintf("%v", ipSet.Ranges())
				expected := "[10.244.1.0-10.244.2.255]"

				if ranges != expected {
					return retry.ExpectedErrorf("ranges %s != expected %s", ranges, expected)
				}

				return nil
			},
		),
	)

	// update config and disable wireguard, everything should be cleaned up
	cfg.TypedSpec().Enabled = false
	suite.Require().NoError(suite.state.Update(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(
				resource.NewMetadata(
					network.ConfigNamespaceName,
					network.LinkSpecType,
					network.LayeredID(network.ConfigOperator, network.LinkID(constants.KubeSpanLinkName)),
					resource.VersionUndefined,
				),
			),
		),
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

	suite.Run(t, new(ManagerSuite))
}
