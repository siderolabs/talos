// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
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
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type OperatorMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *OperatorMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(netctrl.NewOperatorMergeController()))

	suite.startRuntime()
}

func (suite *OperatorMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *OperatorMergeSuite) assertOperators(requiredIDs []string, check func(*network.OperatorSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.OperatorSpecType, "", resource.VersionUndefined),
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

		if err = check(res.(*network.OperatorSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedErrorf("some resources are missing: %q", missingIDs)
	}

	return nil
}

func (suite *OperatorMergeSuite) assertNoOperator(id string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.OperatorSpecType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedErrorf("operator %q is still there", id)
		}
	}

	return nil
}

func (suite *OperatorMergeSuite) TestMerge() {
	dhcp1 := network.NewOperatorSpec(network.ConfigNamespaceName, "default/dhcp4/eth0")
	*dhcp1.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		ConfigLayer: network.ConfigDefault,
	}

	dhcp2 := network.NewOperatorSpec(network.ConfigNamespaceName, "configuration/dhcp4/eth0")
	*dhcp2.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		RequireUp:   true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	dhcp6 := network.NewOperatorSpec(network.ConfigNamespaceName, "configuration/dhcp6/eth0")
	*dhcp6.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP6,
		LinkName:    "eth0",
		RequireUp:   true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{dhcp1, dhcp2, dhcp6} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertOperators(
					[]string{
						"dhcp4/eth0",
						"dhcp6/eth0",
					}, func(r *network.OperatorSpec) error {
						switch r.Metadata().ID() {
						case "dhcp4/eth0":
							suite.Assert().Equal(*dhcp2.TypedSpec(), *r.TypedSpec())
						case "dhcp6/eth0":
							suite.Assert().Equal(*dhcp6.TypedSpec(), *r.TypedSpec())
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, dhcp6.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertOperators(
					[]string{
						"dhcp4/eth0",
					}, func(r *network.OperatorSpec) error {
						return nil
					},
				)
			},
		),
	)
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoOperator("dhcp6/eth0")
			},
		),
	)
}

//nolint:gocyclo
func (suite *OperatorMergeSuite) TestMergeFlapping() {
	// simulate two conflicting operator definitions which are getting removed/added constantly
	dhcp := network.NewOperatorSpec(network.ConfigNamespaceName, "default/dhcp4/eth0")
	*dhcp.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		ConfigLayer: network.ConfigDefault,
	}

	override := network.NewOperatorSpec(network.ConfigNamespaceName, "configuration/dhcp4/eth0")
	*override.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		RequireUp:   true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	resources := []resource.Resource{dhcp, override}

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
						network.OperatorSpecType,
						"dhcp4/eth0",
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
						network.OperatorSpecType,
						"dhcp4/eth0",
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
				return suite.assertOperators(
					[]string{
						"dhcp4/eth0",
					}, func(r *network.OperatorSpec) error {
						if r.Metadata().Phase() != resource.PhaseRunning {
							return retry.ExpectedErrorf("resource phase is %s", r.Metadata().Phase())
						}

						if *override.TypedSpec() != *r.TypedSpec() {
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

func (suite *OperatorMergeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestOperatorMergeSuite(t *testing.T) {
	suite.Run(t, new(OperatorMergeSuite))
}
