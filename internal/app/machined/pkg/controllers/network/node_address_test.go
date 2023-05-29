// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type NodeAddressSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *NodeAddressSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.NodeAddressController{}))

	suite.startRuntime()
}

func (suite *NodeAddressSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *NodeAddressSuite) assertAddresses(requiredIDs []string, check func(*network.NodeAddress) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.NodeAddressType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.NodeAddress)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *NodeAddressSuite) TestDefaults() {
	// create fake device ready status
	deviceStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
	deviceStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.state.Create(suite.ctx, deviceStatus))

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressStatusController{}))
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkStatusController{}))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertAddresses(
					[]string{
						network.NodeAddressDefaultID,
						network.NodeAddressCurrentID,
						network.NodeAddressRoutedID,
						network.NodeAddressAccumulativeID,
					}, func(r *network.NodeAddress) error {
						addrs := r.TypedSpec().Addresses

						suite.T().Logf("id %q val %s", r.Metadata().ID(), addrs)

						suite.Assert().True(
							sort.SliceIsSorted(
								addrs, func(i, j int) bool {
									return addrs[i].Addr().Compare(addrs[j].Addr()) < 0
								},
							), "addresses %s", addrs,
						)

						if r.Metadata().ID() == network.NodeAddressDefaultID {
							if len(addrs) != 1 {
								return fmt.Errorf("there should be only one default address")
							}
						} else {
							if len(addrs) == 0 {
								return fmt.Errorf("there should be some addresses")
							}
						}

						return nil
					},
				)
			},
		),
	)
}

//nolint:gocyclo,cyclop
func (suite *NodeAddressSuite) TestFilters() {
	var (
		addressStatusController  netctrl.AddressStatusController
		platformConfigController netctrl.PlatformConfigController
	)

	linkUp := network.NewLinkStatus(network.NamespaceName, "eth0")
	linkUp.TypedSpec().Type = nethelpers.LinkEther
	linkUp.TypedSpec().LinkState = true
	linkUp.TypedSpec().Index = 1
	suite.Require().NoError(suite.state.Create(suite.ctx, linkUp))

	linkDown := network.NewLinkStatus(network.NamespaceName, "eth1")
	linkDown.TypedSpec().Type = nethelpers.LinkEther
	linkDown.TypedSpec().LinkState = false
	linkDown.TypedSpec().Index = 2
	suite.Require().NoError(suite.state.Create(suite.ctx, linkDown))

	newAddress := func(addr netip.Prefix, link *network.LinkStatus) {
		addressStatus := network.NewAddressStatus(network.NamespaceName, network.AddressID(link.Metadata().ID(), addr))
		addressStatus.TypedSpec().Address = addr
		addressStatus.TypedSpec().LinkName = link.Metadata().ID()
		addressStatus.TypedSpec().LinkIndex = link.TypedSpec().Index
		suite.Require().NoError(
			suite.state.Create(
				suite.ctx,
				addressStatus,
				state.WithCreateOwner(addressStatusController.Name()),
			),
		)
	}

	newExternalAddress := func(addr netip.Prefix) {
		addressStatus := network.NewAddressStatus(network.NamespaceName, network.AddressID("external", addr))
		addressStatus.TypedSpec().Address = addr
		addressStatus.TypedSpec().LinkName = "external"
		suite.Require().NoError(
			suite.state.Create(
				suite.ctx,
				addressStatus,
				state.WithCreateOwner(platformConfigController.Name()),
			),
		)
	}

	for _, addr := range []string{
		"10.0.0.1/8",
		"25.3.7.9/32",
		"2001:470:6d:30e:4a62:b3ba:180b:b5b8/64",
		"127.0.0.1/8",
		"fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128",
	} {
		newAddress(netip.MustParsePrefix(addr), linkUp)
	}

	for _, addr := range []string{"10.0.0.2/8", "192.168.3.7/24"} {
		newAddress(netip.MustParsePrefix(addr), linkDown)
	}

	for _, addr := range []string{"1.2.3.4/32", "25.3.7.9/32"} { // duplicate with link address: 25.3.7.9
		newExternalAddress(netip.MustParsePrefix(addr))
	}

	filter1 := network.NewNodeAddressFilter(network.NamespaceName, "no-k8s")
	filter1.TypedSpec().ExcludeSubnets = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	suite.Require().NoError(suite.state.Create(suite.ctx, filter1))

	filter2 := network.NewNodeAddressFilter(network.NamespaceName, "only-k8s")
	filter2.TypedSpec().IncludeSubnets = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, filter2))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertAddresses(
					[]string{
						network.NodeAddressDefaultID,
						network.NodeAddressCurrentID,
						network.NodeAddressRoutedID,
						network.NodeAddressAccumulativeID,
						network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()),
						network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()),
						network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()),
						network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
						network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()),
						network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()),
					}, func(r *network.NodeAddress) error {
						addrs := r.TypedSpec().Addresses

						switch r.Metadata().ID() {
						case network.NodeAddressDefaultID:
							if !reflect.DeepEqual(addrs, ipList("10.0.0.1/8")) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressCurrentID:
							if !reflect.DeepEqual(
								addrs,
								ipList("1.2.3.4/32 10.0.0.1/8 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressRoutedID:
							if !reflect.DeepEqual(
								addrs,
								ipList("10.0.0.1/8 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressAccumulativeID:
							if !reflect.DeepEqual(
								addrs,
								ipList("1.2.3.4/32 10.0.0.1/8 10.0.0.2/8 25.3.7.9/32 192.168.3.7/24 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()):
							if !reflect.DeepEqual(
								addrs,
								ipList("1.2.3.4/32 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()):
							if !reflect.DeepEqual(
								addrs,
								ipList("25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()):
							if !reflect.DeepEqual(
								addrs,
								ipList("1.2.3.4/32 25.3.7.9/32 192.168.3.7/24 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
							network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()):
							if !reflect.DeepEqual(addrs, ipList("10.0.0.1/8")) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()):
							if !reflect.DeepEqual(addrs, ipList("10.0.0.1/8 10.0.0.2/8 192.168.3.7/24")) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						}

						return nil
					},
				)
			},
		),
	)
}

//nolint:gocyclo
func (suite *NodeAddressSuite) TestDefaultAddressChange() {
	var addressStatusController netctrl.AddressStatusController

	linkUp := network.NewLinkStatus(network.NamespaceName, "eth0")
	linkUp.TypedSpec().Type = nethelpers.LinkEther
	linkUp.TypedSpec().LinkState = true
	linkUp.TypedSpec().Index = 1
	suite.Require().NoError(suite.state.Create(suite.ctx, linkUp))

	newAddress := func(addr netip.Prefix, link *network.LinkStatus) {
		addressStatus := network.NewAddressStatus(network.NamespaceName, network.AddressID(link.Metadata().ID(), addr))
		addressStatus.TypedSpec().Address = addr
		addressStatus.TypedSpec().LinkName = link.Metadata().ID()
		addressStatus.TypedSpec().LinkIndex = link.TypedSpec().Index
		suite.Require().NoError(
			suite.state.Create(
				suite.ctx,
				addressStatus,
				state.WithCreateOwner(addressStatusController.Name()),
			),
		)
	}

	for _, addr := range []string{
		"10.0.0.5/8",
		"25.3.7.9/32",
		"127.0.0.1/8",
	} {
		newAddress(netip.MustParsePrefix(addr), linkUp)
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertAddresses(
					[]string{
						network.NodeAddressDefaultID,
						network.NodeAddressCurrentID,
						network.NodeAddressAccumulativeID,
					}, func(r *network.NodeAddress) error {
						addrs := r.TypedSpec().Addresses

						switch r.Metadata().ID() {
						case network.NodeAddressDefaultID:
							if !reflect.DeepEqual(addrs, ipList("10.0.0.5/8")) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressCurrentID:
							if !reflect.DeepEqual(
								addrs,
								ipList("10.0.0.5/8 25.3.7.9/32"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressAccumulativeID:
							if !reflect.DeepEqual(
								addrs,
								ipList("10.0.0.5/8 25.3.7.9/32"),
							) {
								return fmt.Errorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						}

						return nil
					},
				)
			},
		),
	)

	// add another address which is "smaller", but default address shouldn't change
	newAddress(netip.MustParsePrefix("1.1.1.1/32"), linkUp)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertAddresses(
					[]string{
						network.NodeAddressDefaultID,
						network.NodeAddressCurrentID,
						network.NodeAddressAccumulativeID,
					}, func(r *network.NodeAddress) error {
						addrs := r.TypedSpec().Addresses

						switch r.Metadata().ID() {
						case network.NodeAddressDefaultID:
							if !reflect.DeepEqual(addrs, ipList("10.0.0.5/8")) {
								return retry.ExpectedErrorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressCurrentID:
							if !reflect.DeepEqual(
								addrs,
								ipList("1.1.1.1/32 10.0.0.5/8 25.3.7.9/32"),
							) {
								return retry.ExpectedErrorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressAccumulativeID:
							if !reflect.DeepEqual(
								addrs,
								ipList("1.1.1.1/32 10.0.0.5/8 25.3.7.9/32"),
							) {
								return retry.ExpectedErrorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						}

						return nil
					},
				)
			},
		),
	)

	// remove the previous default address, now default address should change
	suite.Require().NoError(suite.state.Destroy(suite.ctx,
		network.NewAddressStatus(network.NamespaceName, network.AddressID(linkUp.Metadata().ID(), netip.MustParsePrefix("10.0.0.5/8"))).Metadata(),
		state.WithDestroyOwner(addressStatusController.Name()),
	))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertAddresses(
					[]string{
						network.NodeAddressDefaultID,
						network.NodeAddressCurrentID,
						network.NodeAddressAccumulativeID,
					}, func(r *network.NodeAddress) error {
						addrs := r.TypedSpec().Addresses

						switch r.Metadata().ID() {
						case network.NodeAddressDefaultID:
							if !reflect.DeepEqual(addrs, ipList("1.1.1.1/32")) {
								return retry.ExpectedErrorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressCurrentID:
							if !reflect.DeepEqual(
								addrs,
								ipList("1.1.1.1/32 25.3.7.9/32"),
							) {
								return retry.ExpectedErrorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						case network.NodeAddressAccumulativeID:
							if !reflect.DeepEqual(
								addrs,
								ipList("1.1.1.1/32 10.0.0.5/8 25.3.7.9/32"),
							) {
								return retry.ExpectedErrorf("unexpected %q: %s", r.Metadata().ID(), addrs)
							}
						}

						return nil
					},
				)
			},
		),
	)
}

func (suite *NodeAddressSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestNodeAddressSuite(t *testing.T) {
	suite.Run(t, new(NodeAddressSuite))
}

func ipList(ips string) []netip.Prefix {
	var result []netip.Prefix //nolint:prealloc

	for _, ip := range strings.Split(ips, " ") {
		result = append(result, netip.MustParsePrefix(ip))
	}

	return result
}
