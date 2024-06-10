// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"net/netip"
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
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RouteMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *RouteMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
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

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.RouteSpecType, "", resource.VersionUndefined),
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

		if err = check(res.(*network.RouteSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedErrorf("some resources are missing: %q", missingIDs)
	}

	return nil
}

func (suite *RouteMergeSuite) assertNoRoute(id string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.RouteSpecType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedErrorf("address %q is still there", id)
		}
	}

	return nil
}

func (suite *RouteMergeSuite) TestMerge() {
	cmdline := network.NewRouteSpec(network.ConfigNamespaceName, "cmdline/inet4//10.5.0.3/50")
	*cmdline.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigCmdline,
	}

	dhcp := network.NewRouteSpec(network.ConfigNamespaceName, "dhcp/inet4//10.5.0.3/50")
	*dhcp.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewRouteSpec(network.ConfigNamespaceName, "configuration/inet4/10.0.0.35/32/10.0.0.34/1024")
	*static.TypedSpec() = network.RouteSpecSpec{
		Destination: netip.MustParsePrefix("10.0.0.35/32"),
		Gateway:     netip.MustParseAddr("10.0.0.34"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    1024,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{cmdline, dhcp, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoutes(
					[]string{
						"inet4/10.5.0.3//50",
						"inet4/10.0.0.34/10.0.0.35/32/1024",
					}, func(r *network.RouteSpec) error {
						suite.Assert().Equal(resource.PhaseRunning, r.Metadata().Phase())

						switch r.Metadata().ID() {
						case "inet4/10.5.0.3//50":
							suite.Assert().Equal(*dhcp.TypedSpec(), *r.TypedSpec())
						case "inet4/10.0.0.34/10.0.0.35/32/1024":
							suite.Assert().Equal(*static.TypedSpec(), *r.TypedSpec())
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, dhcp.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoutes(
					[]string{
						"inet4/10.5.0.3//50",
						"inet4/10.0.0.34/10.0.0.35/32/1024",
					}, func(r *network.RouteSpec) error {
						suite.Assert().Equal(resource.PhaseRunning, r.Metadata().Phase())

						switch r.Metadata().ID() {
						case "inet4/10.5.0.3//50":
							if *cmdline.TypedSpec() != *r.TypedSpec() {
								// using retry here, as it might not be reconciled immediately
								return retry.ExpectedErrorf("not equal yet")
							}
						case "inet4/10.0.0.34/10.0.0.35/32/1024":
							suite.Assert().Equal(*static.TypedSpec(), *r.TypedSpec())
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoRoute("inet4/10.0.0.34/10.0.0.35/32/1024")
			},
		),
	)
}

//nolint:gocyclo
func (suite *RouteMergeSuite) TestMergeFlapping() {
	// simulate two conflicting default route definitions which are getting removed/added constantly
	cmdline := network.NewRouteSpec(network.ConfigNamespaceName, "cmdline/inet4//10.5.0.3/50")
	*cmdline.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigCmdline,
	}

	dhcp := network.NewRouteSpec(network.ConfigNamespaceName, "dhcp/inet4//10.5.0.3/50")
	*dhcp.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth1",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigOperator,
	}

	resources := []resource.Resource{cmdline, dhcp}

	flipflop := func(idx int) func() error {
		return func() error {
			for range 500 {
				if err := suite.state.Create(suite.ctx, resources[idx]); err != nil {
					return err
				}

				if err := suite.state.Destroy(suite.ctx, resources[idx].Metadata()); err != nil {
					return err
				}

				time.Sleep(time.Millisecond)
			}

			return suite.state.Create(suite.ctx, resources[idx])
		}
	}

	var eg errgroup.Group

	eg.Go(flipflop(0))
	eg.Go(flipflop(1))
	eg.Go(
		func() error {
			// add/remove finalizer to the merged resource
			for range 1000 {
				if err := suite.state.AddFinalizer(
					suite.ctx,
					resource.NewMetadata(
						network.NamespaceName,
						network.RouteSpecType,
						"inet4/10.5.0.3//50",
						resource.VersionUndefined,
					),
					"foo",
				); err != nil {
					if !state.IsNotFoundError(err) {
						return err
					}

					continue
				}

				suite.T().Log("finalizer added")

				time.Sleep(10 * time.Millisecond)

				if err := suite.state.RemoveFinalizer(
					suite.ctx,
					resource.NewMetadata(
						network.NamespaceName,
						network.RouteSpecType,
						"inet4/10.5.0.3//50",
						resource.VersionUndefined,
					),
					"foo",
				); err != nil && !state.IsNotFoundError(err) {
					return err
				}
			}

			return nil
		},
	)

	suite.Require().NoError(eg.Wait())

	suite.Assert().NoError(
		retry.Constant(15*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoutes(
					[]string{
						"inet4/10.5.0.3//50",
					}, func(r *network.RouteSpec) error {
						if r.Metadata().Phase() != resource.PhaseRunning {
							return retry.ExpectedErrorf("resource phase is %s", r.Metadata().Phase())
						}

						if *dhcp.TypedSpec() != *r.TypedSpec() {
							// using retry here, as it might not be reconciled immediately
							return retry.ExpectedErrorf("not equal yet")
						}

						return nil
					},
				)
			},
		),
	)
}

func (suite *RouteMergeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestRouteMergeSuite(t *testing.T) {
	suite.Run(t, new(RouteMergeSuite))
}
