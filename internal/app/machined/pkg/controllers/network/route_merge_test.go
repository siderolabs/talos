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
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/network"
)

type RouteMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *RouteMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.RouteMergeController{}))

	suite.startRuntime()
}

func (suite *RouteMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *RouteMergeSuite) assertRoutes(requiredIDs []string, check func(*network.RouteSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.RouteSpecType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.RouteSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *RouteMergeSuite) assertNoRoute(id string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedError(fmt.Errorf("address %q is still there", id))
		}
	}

	return nil
}

func (suite *RouteMergeSuite) TestMerge() {
	cmdline := network.NewRouteSpec(network.ConfigNamespaceName, "cmdline//10.5.0.3")
	*cmdline.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netaddr.MustParseIP("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Priority:    1024,
		ConfigLayer: network.ConfigCmdline,
	}

	dhcp := network.NewRouteSpec(network.ConfigNamespaceName, "dhcp//10.5.0.3")
	*dhcp.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netaddr.MustParseIP("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Priority:    50,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewRouteSpec(network.ConfigNamespaceName, "configuration/10.0.0.35/32/10.0.0.34")
	*static.TypedSpec() = network.RouteSpecSpec{
		Destination: netaddr.MustParseIPPrefix("10.0.0.35/32"),
		Gateway:     netaddr.MustParseIP("10.0.0.34"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Priority:    1024,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{cmdline, dhcp, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertRoutes([]string{
				"/10.5.0.3",
				"10.0.0.35/32/10.0.0.34",
			}, func(r *network.RouteSpec) error {
				switch r.Metadata().ID() {
				case "/10.5.0.3":
					suite.Assert().Equal(*dhcp.TypedSpec(), *r.TypedSpec())
				case "10.0.0.35/32/10.0.0.34":
					suite.Assert().Equal(*static.TypedSpec(), *r.TypedSpec())
				}

				return nil
			})
		}))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, dhcp.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertRoutes([]string{
				"/10.5.0.3",
				"10.0.0.35/32/10.0.0.34",
			}, func(r *network.RouteSpec) error {
				switch r.Metadata().ID() {
				case "/10.5.0.3":
					if *cmdline.TypedSpec() != *r.TypedSpec() {
						// using retry here, as it might not be reconciled immediately
						return retry.ExpectedError(fmt.Errorf("not equal yet"))
					}
				case "10.0.0.35/32/10.0.0.34":
					suite.Assert().Equal(*static.TypedSpec(), *r.TypedSpec())
				}

				return nil
			})
		}))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoRoute("10.0.0.35/32/10.0.0.34")
		}))
}

func TestRouteMergeSuite(t *testing.T) {
	suite.Run(t, new(RouteMergeSuite))
}
