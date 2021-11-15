// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
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

type ResolverSpecSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *ResolverSpecSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.ResolverSpecController{}))

	suite.startRuntime()
}

func (suite *ResolverSpecSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *ResolverSpecSuite) assertStatus(id string, servers ...netaddr.IP) error {
	r, err := suite.state.Get(suite.ctx, resource.NewMetadata(network.NamespaceName, network.ResolverStatusType, id, resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return retry.ExpectedError(err)
		}

		return err
	}

	status := r.(*network.ResolverStatus) //nolint:errcheck,forcetypeassert

	if !reflect.DeepEqual(status.TypedSpec().DNSServers, servers) {
		return retry.ExpectedErrorf("server list mismatch: %q != %q", status.TypedSpec().DNSServers, servers)
	}

	return nil
}

func (suite *ResolverSpecSuite) TestSpec() {
	spec := network.NewResolverSpec(network.NamespaceName, "resolvers")
	*spec.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netaddr.IP{netaddr.MustParseIP(constants.DefaultPrimaryResolver)},
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{spec} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertStatus("resolvers", netaddr.MustParseIP(constants.DefaultPrimaryResolver))
		}))
}

func (suite *ResolverSpecSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewResolverSpec(network.NamespaceName, "bar")))
}

func TestResolverSpecSuite(t *testing.T) {
	suite.Run(t, new(ResolverSpecSuite))
}
