// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type AddressSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *AddressSpecSuite) uniqueDummyInterface() string {
	return fmt.Sprintf("dummy%02x%02x%02x", rand.Int32()&0xff, rand.Int32()&0xff, rand.Int32()&0xff)
}

func assertLinkAddress(asrt *assert.Assertions, linkName, address string) {
	addr := netip.MustParsePrefix(address)

	iface, err := net.InterfaceByName(linkName)
	asrt.NoError(err)

	conn, err := rtnetlink.Dial(nil)
	asrt.NoError(err)

	defer conn.Close() //nolint:errcheck

	linkAddresses, err := conn.Address.List()
	asrt.NoError(err)

	for _, linkAddress := range linkAddresses {
		if linkAddress.Index != uint32(iface.Index) {
			continue
		}

		if int(linkAddress.PrefixLength) != addr.Bits() {
			continue
		}

		if !linkAddress.Attributes.Address.Equal(addr.Addr().AsSlice()) {
			continue
		}

		return
	}

	asrt.Failf("address not found", "address %s not found on %q", addr, linkName)
}

func assertNoLinkAddress(asrt *assert.Assertions, linkName, address string) {
	addr := netip.MustParsePrefix(address)

	iface, err := net.InterfaceByName(linkName)
	asrt.NoError(err)

	conn, err := rtnetlink.Dial(nil)
	asrt.NoError(err)

	defer conn.Close() //nolint:errcheck

	linkAddresses, err := conn.Address.List()
	asrt.NoError(err)

	for _, linkAddress := range linkAddresses {
		if linkAddress.Index == uint32(iface.Index) && int(linkAddress.PrefixLength) == addr.Bits() && linkAddress.Attributes.Address.Equal(addr.Addr().AsSlice()) {
			asrt.Failf("address is still there", "address %s is assigned to %q", addr, linkName)
		}
	}
}

func (suite *AddressSpecSuite) TestLoopback() {
	loopback := network.NewAddressSpec(network.NamespaceName, "lo/127.0.0.1/8")
	*loopback.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("127.11.0.1/32"),
		LinkName:    "lo",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeHost,
		ConfigLayer: network.ConfigDefault,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
	}

	for _, res := range []resource.Resource{loopback} {
		suite.Create(res)
	}

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assertLinkAddress(assert.New(collect), "lo", "127.11.0.1/32")
	}, 3*time.Second, 10*time.Millisecond)

	// teardown the address
	_, err := suite.State().Teardown(suite.Ctx(), loopback.Metadata())
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), loopback.Metadata(), state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	// torn down address should be removed immediately
	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assertNoLinkAddress(assert.New(collect), "lo", "127.11.0.1/32")
	}, 3*time.Second, 10*time.Millisecond)

	suite.Destroy(loopback)
}

func (suite *AddressSpecSuite) TestDummy() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	dummy := network.NewAddressSpec(network.NamespaceName, "dummy/10.0.0.1/8")
	*dummy.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.1/8"),
		LinkName:    dummyInterface,
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigDefault,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
	}

	// it's fine to create the address before the interface is actually created
	for _, res := range []resource.Resource{dummy} {
		suite.Create(res)
	}

	// create dummy interface
	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type: unix.ARPHRD_ETHER,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					MTU:  1400,
					Info: &rtnetlink.LinkInfo{
						Kind: "dummy",
					},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assertLinkAddress(assert.New(collect), dummyInterface, "10.0.0.1/8")
	}, 3*time.Second, 10*time.Millisecond)

	// delete dummy interface, address should be unassigned automatically
	suite.Require().NoError(conn.Link.Delete(uint32(iface.Index)))

	// teardown the address
	_, err = suite.State().Teardown(suite.Ctx(), dummy.Metadata())
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), dummy.Metadata(), state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Destroy(dummy)
}

func (suite *AddressSpecSuite) TestDummyAlias() {
	dummyInterface := suite.uniqueDummyInterface()
	dummyAlias := suite.uniqueDummyInterface()

	suite.T().Logf("dummyInterface: %s, dummyAlias: %s", dummyInterface, dummyAlias)

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	dummy := network.NewAddressSpec(network.NamespaceName, "dummy/10.0.0.5/8")
	*dummy.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.5/8"),
		LinkName:    dummyAlias, // use alias name instead of the actual interface name
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigDefault,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
	}

	// it's fine to create the address before the interface is actually created
	for _, res := range []resource.Resource{dummy} {
		suite.Create(res)
	}

	// create dummy interface
	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type: unix.ARPHRD_ETHER,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					MTU:  1400,
					Info: &rtnetlink.LinkInfo{
						Kind: "dummy",
					},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	// set alias name
	suite.Require().NoError(
		conn.Link.Set(
			&rtnetlink.LinkMessage{
				Index: uint32(iface.Index),
				Attributes: &rtnetlink.LinkAttributes{
					Alias: &dummyAlias,
				},
			},
		),
	)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assertLinkAddress(assert.New(collect), dummyInterface, "10.0.0.5/8")
	}, 3*time.Second, 10*time.Millisecond)
}

func TestAddressSpecSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, &AddressSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.AddressSpecController{}))
			},
		},
	})
}
