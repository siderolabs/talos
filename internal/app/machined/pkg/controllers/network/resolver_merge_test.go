// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"reflect"
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
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type ResolverMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *ResolverMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.ResolverMergeController{}))

	suite.startRuntime()
}

func (suite *ResolverMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *ResolverMergeSuite) assertResolvers(requiredIDs []string, check func(*network.ResolverSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.ResolverSpecType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.ResolverSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *ResolverMergeSuite) TestMerge() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netaddr.IP{netaddr.MustParseIP(constants.DefaultPrimaryResolver), netaddr.MustParseIP(constants.DefaultSecondaryResolver)},
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netaddr.IP{netaddr.MustParseIP("1.1.2.0")},
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netaddr.IP{netaddr.MustParseIP("1.1.2.1")},
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewResolverSpec(network.ConfigNamespaceName, "configuration/resolvers")
	*static.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netaddr.IP{netaddr.MustParseIP("2.2.2.2")},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResolvers([]string{
				"resolvers",
			}, func(r *network.ResolverSpec) error {
				suite.Assert().Equal(*static.TypedSpec(), *r.TypedSpec())

				return nil
			})
		}))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResolvers([]string{
				"resolvers",
			}, func(r *network.ResolverSpec) error {
				if !reflect.DeepEqual(r.TypedSpec().DNSServers, []netaddr.IP{netaddr.MustParseIP("1.1.2.0"), netaddr.MustParseIP("1.1.2.1")}) {
					return retry.ExpectedErrorf("unexpected servers %q", r.TypedSpec().DNSServers)
				}

				return nil
			})
		}))
}

func (suite *ResolverMergeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewResolverSpec(network.ConfigNamespaceName, "bar")))
}

func TestResolverMergeSuite(t *testing.T) {
	suite.Run(t, new(ResolverMergeSuite))
}
