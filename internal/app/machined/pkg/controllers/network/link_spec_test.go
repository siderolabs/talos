// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	networkadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/network"
	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type LinkSpecSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *LinkSpecSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkSpecController{}))

	// register status controller to assert on the created links
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkStatusController{}))

	suite.startRuntime()
}

func (suite *LinkSpecSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *LinkSpecSuite) assertInterfaces(requiredIDs []string, check func(*network.LinkStatus) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.LinkStatus)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *LinkSpecSuite) assertNoInterface(id string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedError(fmt.Errorf("interface %q is still there", id))
		}
	}

	return nil
}

func (suite *LinkSpecSuite) uniqueDummyInterface() string {
	return fmt.Sprintf("dummy%02x%02x%02x", rand.Int31()&0xff, rand.Int31()&0xff, rand.Int31()&0xff)
}

func (suite *LinkSpecSuite) TestLoopback() {
	loopback := network.NewLinkSpec(network.NamespaceName, "lo")
	*loopback.TypedSpec() = network.LinkSpecSpec{
		Name:        "lo",
		Up:          true,
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{loopback} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{"lo"}, func(r *network.LinkStatus) error {
				return nil
			})
		}))
}

func (suite *LinkSpecSuite) TestDummy() {
	dummyInterface := suite.uniqueDummyInterface()

	dummy := network.NewLinkSpec(network.NamespaceName, dummyInterface)
	*dummy.TypedSpec() = network.LinkSpecSpec{
		Name:        dummyInterface,
		Type:        nethelpers.LinkEther,
		Kind:        "dummy",
		MTU:         1400,
		Up:          true,
		Logical:     true,
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{dummy} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{dummyInterface}, func(r *network.LinkStatus) error {
				suite.Assert().Equal("dummy", r.TypedSpec().Kind)

				if r.TypedSpec().OperationalState != nethelpers.OperStateUnknown && r.TypedSpec().OperationalState != nethelpers.OperStateUp {
					return retry.ExpectedErrorf("link is not up")
				}

				if r.TypedSpec().MTU != 1400 {
					return retry.ExpectedErrorf("unexpected MTU %d", r.TypedSpec().MTU)
				}

				return nil
			})
		}))

	// teardown the link
	for {
		ready, err := suite.state.Teardown(suite.ctx, dummy.Metadata())
		suite.Require().NoError(err)

		if ready {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoInterface(dummyInterface)
		}))
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestVLAN() {
	dummyInterface := suite.uniqueDummyInterface()

	dummy := network.NewLinkSpec(network.NamespaceName, dummyInterface)
	*dummy.TypedSpec() = network.LinkSpecSpec{
		Name:        dummyInterface,
		Type:        nethelpers.LinkEther,
		Kind:        "dummy",
		Up:          true,
		Logical:     true,
		ConfigLayer: network.ConfigDefault,
	}

	vlanName1 := fmt.Sprintf("%s.%d", dummyInterface, 2)
	vlan1 := network.NewLinkSpec(network.NamespaceName, vlanName1)
	*vlan1.TypedSpec() = network.LinkSpecSpec{
		Name:        vlanName1,
		Type:        nethelpers.LinkEther,
		Kind:        network.LinkKindVLAN,
		Up:          true,
		Logical:     true,
		ParentName:  dummyInterface,
		ConfigLayer: network.ConfigDefault,
		VLAN: network.VLANSpec{
			VID:      2,
			Protocol: nethelpers.VLANProtocol8021Q,
		},
	}

	vlanName2 := fmt.Sprintf("%s.%d", dummyInterface, 4)
	vlan2 := network.NewLinkSpec(network.NamespaceName, vlanName2)
	*vlan2.TypedSpec() = network.LinkSpecSpec{
		Name:        vlanName2,
		Type:        nethelpers.LinkEther,
		Kind:        network.LinkKindVLAN,
		Up:          true,
		Logical:     true,
		ParentName:  dummyInterface,
		ConfigLayer: network.ConfigDefault,
		VLAN: network.VLANSpec{
			VID:      4,
			Protocol: nethelpers.VLANProtocol8021Q,
		},
	}

	for _, res := range []resource.Resource{dummy, vlan1, vlan2} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{dummyInterface, vlanName1, vlanName2}, func(r *network.LinkStatus) error {
				switch r.Metadata().ID() {
				case dummyInterface:
					suite.Assert().Equal("dummy", r.TypedSpec().Kind)
				case vlanName1, vlanName2:
					suite.Assert().Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
					suite.Assert().Equal(nethelpers.VLANProtocol8021Q, r.TypedSpec().VLAN.Protocol)

					if r.Metadata().ID() == vlanName1 {
						suite.Assert().EqualValues(2, r.TypedSpec().VLAN.VID)
					} else {
						suite.Assert().EqualValues(4, r.TypedSpec().VLAN.VID)
					}
				}

				if r.TypedSpec().OperationalState != nethelpers.OperStateUnknown && r.TypedSpec().OperationalState != nethelpers.OperStateUp {
					return retry.ExpectedErrorf("link is not up")
				}

				return nil
			})
		}))

	// attempt to change VLAN ID
	_, err := suite.state.UpdateWithConflicts(suite.ctx, vlan1.Metadata(), func(r resource.Resource) error {
		r.(*network.LinkSpec).TypedSpec().VLAN.VID = 42

		return nil
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{vlanName1}, func(r *network.LinkStatus) error {
				if r.TypedSpec().VLAN.VID != 42 {
					return retry.ExpectedErrorf("vlan ID is not 42: %d", r.TypedSpec().VLAN.VID)
				}

				return nil
			})
		}))

	// teardown the links
	for _, r := range []resource.Resource{vlan1, vlan2, dummy} {
		for {
			ready, err := suite.state.Teardown(suite.ctx, r.Metadata())
			suite.Require().NoError(err)

			if ready {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoInterface(dummyInterface)
		}))
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestBond() {
	bondName := suite.uniqueDummyInterface()
	bond := network.NewLinkSpec(network.NamespaceName, bondName)
	*bond.TypedSpec() = network.LinkSpecSpec{
		Name:    bondName,
		Type:    nethelpers.LinkEther,
		Kind:    network.LinkKindBond,
		Up:      true,
		Logical: true,
		BondMaster: network.BondMasterSpec{
			Mode:            nethelpers.BondModeActiveBackup,
			ARPAllTargets:   nethelpers.ARPAllTargetsAll,
			PrimaryReselect: nethelpers.PrimaryReselectBetter,
			FailOverMac:     nethelpers.FailOverMACFollow,
			ADSelect:        nethelpers.ADSelectBandwidth,
			MIIMon:          100,
			DownDelay:       100,
			ResendIGMP:      2,
			UseCarrier:      true,
		},
		ConfigLayer: network.ConfigDefault,
	}
	networkadapter.BondMasterSpec(&bond.TypedSpec().BondMaster).FillDefaults()

	dummy0Name := suite.uniqueDummyInterface()
	dummy0 := network.NewLinkSpec(network.NamespaceName, dummy0Name)
	*dummy0.TypedSpec() = network.LinkSpecSpec{
		Name:        dummy0Name,
		Type:        nethelpers.LinkEther,
		Kind:        "dummy",
		Up:          true,
		Logical:     true,
		MasterName:  bondName,
		ConfigLayer: network.ConfigDefault,
	}

	dummy1Name := suite.uniqueDummyInterface()
	dummy1 := network.NewLinkSpec(network.NamespaceName, dummy1Name)
	*dummy1.TypedSpec() = network.LinkSpecSpec{
		Name:        dummy1Name,
		Type:        nethelpers.LinkEther,
		Kind:        "dummy",
		Up:          true,
		Logical:     true,
		MasterName:  bondName,
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{dummy0, dummy1, bond} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{dummy0Name, dummy1Name, bondName}, func(r *network.LinkStatus) error {
				switch r.Metadata().ID() {
				case bondName:
					suite.Assert().Equal(network.LinkKindBond, r.TypedSpec().Kind)

					if r.TypedSpec().OperationalState != nethelpers.OperStateUnknown && r.TypedSpec().OperationalState != nethelpers.OperStateUp {
						return retry.ExpectedErrorf("link is not up: %s", r.TypedSpec().OperationalState)
					}
				case dummy0Name, dummy1Name:
					suite.Assert().Equal("dummy", r.TypedSpec().Kind)

					if r.TypedSpec().OperationalState != nethelpers.OperStateUnknown {
						return retry.ExpectedErrorf("link is not up: %s", r.TypedSpec().OperationalState)
					}

					if r.TypedSpec().MasterIndex == 0 {
						return retry.ExpectedErrorf("masterIndex should be non-zero")
					}
				}

				return nil
			})
		}))

	// attempt to change bond type
	_, err := suite.state.UpdateWithConflicts(suite.ctx, bond.Metadata(), func(r resource.Resource) error {
		r.(*network.LinkSpec).TypedSpec().BondMaster.Mode = nethelpers.BondModeRoundrobin

		return nil
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{bondName}, func(r *network.LinkStatus) error {
				if r.TypedSpec().BondMaster.Mode != nethelpers.BondModeRoundrobin {
					return retry.ExpectedErrorf("bond mode is not %s: %s", nethelpers.BondModeRoundrobin, r.TypedSpec().BondMaster.Mode)
				}

				return nil
			})
		}))

	// unslave one of the interfaces
	_, err = suite.state.UpdateWithConflicts(suite.ctx, dummy0.Metadata(), func(r resource.Resource) error {
		r.(*network.LinkSpec).TypedSpec().MasterName = ""

		return nil
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{dummy0Name}, func(r *network.LinkStatus) error {
				if r.TypedSpec().MasterIndex != 0 {
					return retry.ExpectedErrorf("iface not unslaved yet")
				}

				return nil
			})
		}))

	// teardown the links
	for _, r := range []resource.Resource{dummy0, dummy1, bond} {
		for {
			ready, err := suite.state.Teardown(suite.ctx, r.Metadata())
			suite.Require().NoError(err)

			if ready {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoInterface(bondName)
		}))
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestBond8023ad() {
	bondName := suite.uniqueDummyInterface()
	bond := network.NewLinkSpec(network.NamespaceName, bondName)
	*bond.TypedSpec() = network.LinkSpecSpec{
		Name:    bondName,
		Type:    nethelpers.LinkEther,
		Kind:    network.LinkKindBond,
		MTU:     9000,
		Up:      true,
		Logical: true,
		BondMaster: network.BondMasterSpec{
			Mode:       nethelpers.BondMode8023AD,
			LACPRate:   nethelpers.LACPRateFast,
			UseCarrier: true,
		},
		ConfigLayer: network.ConfigDefault,
	}
	networkadapter.BondMasterSpec(&bond.TypedSpec().BondMaster).FillDefaults()

	dummies := []resource.Resource{}
	dummyNames := []string{}

	for i := 0; i < 4; i++ {
		dummyName := suite.uniqueDummyInterface()
		dummy := network.NewLinkSpec(network.NamespaceName, dummyName)
		*dummy.TypedSpec() = network.LinkSpecSpec{
			Name:        dummyName,
			Type:        nethelpers.LinkEther,
			Kind:        "dummy",
			Up:          true,
			Logical:     true,
			MasterName:  bondName,
			ConfigLayer: network.ConfigDefault,
		}

		dummies = append(dummies, dummy)
		dummyNames = append(dummyNames, dummyName)
	}

	for _, res := range append(dummies, bond) {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces(append(dummyNames, bondName), func(r *network.LinkStatus) error {
				if r.Metadata().ID() == bondName {
					// master
					suite.Assert().Equal(network.LinkKindBond, r.TypedSpec().Kind)

					if r.TypedSpec().OperationalState != nethelpers.OperStateUnknown && r.TypedSpec().OperationalState != nethelpers.OperStateUp {
						return retry.ExpectedErrorf("link is not up: %s", r.TypedSpec().OperationalState)
					}
				} else {
					// slaves
					suite.Assert().Equal("dummy", r.TypedSpec().Kind)

					if r.TypedSpec().OperationalState != nethelpers.OperStateUnknown {
						return retry.ExpectedErrorf("link is not up: %s", r.TypedSpec().OperationalState)
					}

					if r.TypedSpec().MasterIndex == 0 {
						return retry.ExpectedErrorf("masterIndex should be non-zero")
					}
				}

				return nil
			})
		}))

	// teardown the links
	for _, r := range append(dummies, bond) {
		for {
			ready, err := suite.state.Teardown(suite.ctx, r.Metadata())
			suite.Require().NoError(err)

			if ready {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoInterface(bondName)
		}))
}

func (suite *LinkSpecSuite) TestWireguard() {
	priv, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	pub1, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	pub2, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	wgInterface := suite.uniqueDummyInterface()

	wg := network.NewLinkSpec(network.NamespaceName, wgInterface)
	*wg.TypedSpec() = network.LinkSpecSpec{
		Name:    wgInterface,
		Type:    nethelpers.LinkNone,
		Kind:    "wireguard",
		Up:      true,
		Logical: true,
		Wireguard: network.WireguardSpec{
			PrivateKey:   priv.String(),
			ListenPort:   30000,
			FirewallMark: 1,
			Peers: []network.WireguardPeer{
				{
					PublicKey: pub1.PublicKey().String(),
					Endpoint:  "10.2.0.3:20000",
					AllowedIPs: []netaddr.IPPrefix{
						netaddr.MustParseIPPrefix("172.24.0.0/16"),
					},
				},
				{
					PublicKey: pub2.PublicKey().String(),
					AllowedIPs: []netaddr.IPPrefix{
						netaddr.MustParseIPPrefix("172.25.0.0/24"),
					},
				},
			},
		},
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{wg} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{wgInterface}, func(r *network.LinkStatus) error {
				suite.Assert().Equal("wireguard", r.TypedSpec().Kind)

				if r.TypedSpec().Wireguard.PublicKey != priv.PublicKey().String() {
					return retry.ExpectedErrorf("private key not set")
				}

				if len(r.TypedSpec().Wireguard.Peers) != 2 {
					return retry.ExpectedErrorf("peers are not set up")
				}

				if r.TypedSpec().OperationalState != nethelpers.OperStateUnknown && r.TypedSpec().OperationalState != nethelpers.OperStateUp {
					return retry.ExpectedErrorf("link is not up")
				}

				return nil
			})
		}))

	// attempt to change wireguard private key
	priv2, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	_, err = suite.state.UpdateWithConflicts(suite.ctx, wg.Metadata(), func(r resource.Resource) error {
		r.(*network.LinkSpec).TypedSpec().Wireguard.PrivateKey = priv2.String()

		return nil
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertInterfaces([]string{wgInterface}, func(r *network.LinkStatus) error {
				if r.TypedSpec().Wireguard.PublicKey != priv2.PublicKey().String() {
					return retry.ExpectedErrorf("private key was not updated")
				}

				return nil
			})
		}))

	// teardown the links
	for _, r := range []resource.Resource{wg} {
		for {
			ready, err := suite.state.Teardown(suite.ctx, r.Metadata())
			suite.Require().NoError(err)

			if ready {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoInterface(wgInterface)
		}))
}

func (suite *LinkSpecSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewLinkSpec(network.NamespaceName, "bar")))
}

func TestLinkSpecSuite(t *testing.T) {
	suite.Run(t, new(LinkSpecSuite))
}
