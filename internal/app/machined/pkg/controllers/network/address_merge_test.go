// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
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
	"inet.af/netaddr"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/resources/network"
	"github.com/talos-systems/talos/pkg/resources/network/nethelpers"
)

type AddressMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *AddressMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressMergeController{}))

	suite.startRuntime()
}

func (suite *AddressMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *AddressMergeSuite) assertAddresses(requiredIDs []string, check func(*network.AddressSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.AddressSpecType, "", resource.VersionUndefined))
	if err != nil {
		return retry.UnexpectedError(err)
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.AddressSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *AddressMergeSuite) assertNoAddress(id string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined))
	if err != nil {
		return retry.UnexpectedError(err)
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedError(fmt.Errorf("address %q is still there", id))
		}
	}

	return nil
}

func (suite *AddressMergeSuite) TestMerge() {
	loopback := network.NewAddressSpec(network.ConfigNamespaceName, "default/lo/127.0.0.1/8")
	*loopback.Status() = network.AddressSpecSpec{
		Address:  netaddr.MustParseIPPrefix("127.0.0.1/8"),
		LinkName: "lo",
		Family:   nethelpers.FamilyInet4,
		Scope:    nethelpers.ScopeHost,
		Layer:    network.ConfigDefault,
	}

	dhcp := network.NewAddressSpec(network.ConfigNamespaceName, "dhcp/eth0/10.0.0.1/8")
	*dhcp.Status() = network.AddressSpecSpec{
		Address:  netaddr.MustParseIPPrefix("10.0.0.1/8"),
		LinkName: "eth0",
		Family:   nethelpers.FamilyInet4,
		Scope:    nethelpers.ScopeGlobal,
		Layer:    network.ConfigDHCP,
	}

	static := network.NewAddressSpec(network.ConfigNamespaceName, "configuration/eth0/10.0.0.35/32")
	*static.Status() = network.AddressSpecSpec{
		Address:  netaddr.MustParseIPPrefix("10.0.0.35/32"),
		LinkName: "eth0",
		Family:   nethelpers.FamilyInet4,
		Scope:    nethelpers.ScopeGlobal,
		Layer:    network.ConfigMachineConfiguration,
	}

	override := network.NewAddressSpec(network.ConfigNamespaceName, "configuration/eth0/10.0.0.1/8")
	*override.Status() = network.AddressSpecSpec{
		Address:  netaddr.MustParseIPPrefix("10.0.0.1/8"),
		LinkName: "eth0",
		Family:   nethelpers.FamilyInet4,
		Scope:    nethelpers.ScopeHost,
		Layer:    network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{loopback, dhcp, static, override} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertAddresses([]string{
				"lo/127.0.0.1/8",
				"eth0/10.0.0.1/8",
				"eth0/10.0.0.35/32",
			}, func(r *network.AddressSpec) error {
				switch r.Metadata().ID() {
				case "lo/127.0.0.1/8":
					suite.Assert().Equal(*loopback.Status(), *r.Status())
				case "eth0/10.0.0.1/8":
					suite.Assert().Equal(*override.Status(), *r.Status())
				case "eth0/10.0.0.35/32":
					suite.Assert().Equal(*static.Status(), *r.Status())
				}

				return nil
			})
		}))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertAddresses([]string{
				"lo/127.0.0.1/8",
				"eth0/10.0.0.35/32",
			}, func(r *network.AddressSpec) error {
				return nil
			})
		}))
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoAddress("eth0/10.0.0.35/32")
		}))
}

func TestAddressMergeSuite(t *testing.T) {
	suite.Run(t, new(AddressMergeSuite))
}
