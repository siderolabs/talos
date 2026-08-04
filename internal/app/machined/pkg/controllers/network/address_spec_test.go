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
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	if findLinkAddress(asrt, linkName, address) == nil {
		asrt.Failf("address not found", "address %s not found on %q", address, linkName)
	}
}

func assertNoLinkAddress(asrt *assert.Assertions, linkName, address string) {
	if findLinkAddress(asrt, linkName, address) != nil {
		asrt.Failf("address is still there", "address %s is assigned to %q", address, linkName)
	}
}

// findLinkAddress returns the address as assigned to the link, or nil if it is not assigned.
func findLinkAddress(asrt *assert.Assertions, linkName, address string) *rtnetlink.AddressMessage {
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
			return &linkAddress
		}
	}

	return nil
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

func (suite *AddressSpecSuite) TestIPV6ULA() {
	loopback := network.NewAddressSpec(network.NamespaceName, "lo/"+constants.HostDNSAddressV6+"/128")
	*loopback.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix(constants.HostDNSAddressV6 + "/128"),
		LinkName:    "lo",
		Family:      nethelpers.FamilyInet6,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigDefault,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
	}

	for _, res := range []resource.Resource{loopback} {
		suite.Create(res)
	}

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assertLinkAddress(assert.New(collect), "lo", constants.HostDNSAddressV6+"/128")
	}, 3*time.Second, 10*time.Millisecond)

	// teardown the address
	_, err := suite.State().Teardown(suite.Ctx(), loopback.Metadata())
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), loopback.Metadata(), state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	// torn down address should be removed immediately
	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assertNoLinkAddress(assert.New(collect), "lo", constants.HostDNSAddressV6+"/128")
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

// TestDummySecondary verifies that the addresses sharing the subnet are not being re-created in a loop:
// the kernel marks all but the first IPv4 address in the subnet as IFA_F_SECONDARY, and Talos should not
// try to enforce the flags it doesn't manage.
func (suite *AddressSpecSuite) TestDummySecondary() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

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

	addresses := []string{"10.99.0.1/24", "10.99.0.2/24"}

	specs := xslices.Map(addresses, func(address string) *network.AddressSpec {
		spec := network.NewAddressSpec(network.NamespaceName, dummyInterface+"/"+address)
		*spec.TypedSpec() = network.AddressSpecSpec{
			Address:     netip.MustParsePrefix(address),
			LinkName:    dummyInterface,
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeGlobal,
			ConfigLayer: network.ConfigDefault,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		}

		suite.Create(spec)

		return spec
	})

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		for _, address := range addresses {
			assertLinkAddress(assert.New(collect), dummyInterface, address)
		}
	}, 3*time.Second, 10*time.Millisecond)

	// the kernel should have marked the second address as secondary
	secondary := findLinkAddress(suite.Assert(), dummyInterface, addresses[1])
	suite.Require().NotNil(secondary)
	suite.Assert().NotZero(secondary.Attributes.Flags & uint32(nethelpers.AddressTemporary))

	// remember the creation timestamps of the addresses, they should not change as the addresses
	// should not be re-created
	created := xslices.Map(addresses, func(address string) uint32 {
		addr := findLinkAddress(suite.Assert(), dummyInterface, address)
		suite.Require().NotNil(addr)

		return addr.Attributes.CacheInfo.Created
	})

	// force the controller to reconcile the addresses a couple of times
	for range 3 {
		for _, spec := range specs {
			res, err := suite.State().Get(suite.Ctx(), spec.Metadata())
			suite.Require().NoError(err)

			suite.Update(res)
		}

		time.Sleep(100 * time.Millisecond)
	}

	for i, address := range addresses {
		addr := findLinkAddress(suite.Assert(), dummyInterface, address)
		suite.Require().NotNil(addr)
		suite.Assert().Equalf(created[i], addr.Attributes.CacheInfo.Created, "address %s was re-created", address)
	}
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
