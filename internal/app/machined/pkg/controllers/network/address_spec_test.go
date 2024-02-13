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
	"net"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/jsimonetti/rtnetlink"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type AddressSpecSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *AddressSpecSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressSpecController{}))

	suite.startRuntime()
}

func (suite *AddressSpecSuite) uniqueDummyInterface() string {
	return fmt.Sprintf("dummy%02x%02x%02x", rand.Int31()&0xff, rand.Int31()&0xff, rand.Int31()&0xff)
}

func (suite *AddressSpecSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *AddressSpecSuite) assertLinkAddress(linkName, address string) error {
	addr := netip.MustParsePrefix(address)

	iface, err := net.InterfaceByName(linkName)
	suite.Require().NoError(err)

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	linkAddresses, err := conn.Address.List()
	suite.Require().NoError(err)

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

		return nil
	}

	return retry.ExpectedErrorf("address %s not found on %q", addr, linkName)
}

func (suite *AddressSpecSuite) assertNoLinkAddress(linkName, address string) error {
	addr := netip.MustParsePrefix(address)

	iface, err := net.InterfaceByName(linkName)
	suite.Require().NoError(err)

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	linkAddresses, err := conn.Address.List()
	suite.Require().NoError(err)

	for _, linkAddress := range linkAddresses {
		if linkAddress.Index == uint32(iface.Index) && int(linkAddress.PrefixLength) == addr.Bits() && linkAddress.Attributes.Address.Equal(addr.Addr().AsSlice()) {
			return retry.ExpectedErrorf("address %s is assigned to %q", addr, linkName)
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
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertLinkAddress("lo", "127.11.0.1/32")
			},
		),
	)

	// teardown the address
	for {
		ready, err := suite.state.Teardown(suite.ctx, loopback.Metadata())
		suite.Require().NoError(err)

		if ready {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	// torn down address should be removed immediately
	suite.Assert().NoError(suite.assertNoLinkAddress("lo", "127.11.0.1/32"))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, loopback.Metadata()))
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
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
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

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertLinkAddress(dummyInterface, "10.0.0.1/8")
			},
		),
	)

	// delete dummy interface, address should be unassigned automatically
	suite.Require().NoError(conn.Link.Delete(uint32(iface.Index)))

	// teardown the address
	for {
		ready, err := suite.state.Teardown(suite.ctx, dummy.Metadata())
		suite.Require().NoError(err)

		if ready {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (suite *AddressSpecSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestAddressSpecSuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, new(AddressSpecSuite))
}
