// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

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

	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type NodeIPSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *NodeIPSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.NodeIPController{}))

	suite.startRuntime()
}

func (suite *NodeIPSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *NodeIPSuite) TestReconcileIPv4() {
	suite.T().Skip("skipping as the code relies on net.IPAddrs")

	cfg := k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID)

	cfg.TypedSpec().ValidSubnets = []string{"10.0.0.0/24", "::/0"}
	cfg.TypedSpec().ExcludeSubnets = []string{"10.0.0.2"}

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	addresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s))

	addresses.TypedSpec().Addresses = []netaddr.IPPrefix{
		netaddr.MustParseIPPrefix("10.0.0.2/32"), // excluded explicitly
		netaddr.MustParseIPPrefix("10.0.0.5/24"),
		netaddr.MustParseIPPrefix("fdae:41e4:649b:9303::1/64"), // SideroLink IP
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, addresses))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			NodeIP, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodeIPType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := NodeIP.(*k8s.NodeIP).TypedSpec()

			suite.Assert().Equal("[10.0.0.5]", fmt.Sprintf("%s", spec.Addresses))

			return nil
		},
	))
}

func (suite *NodeIPSuite) TestReconcileDefaultSubnets() {
	suite.T().Skip("skipping as the code relies on net.IPAddrs")

	cfg := k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID)

	cfg.TypedSpec().ValidSubnets = []string{"0.0.0.0/0", "::/0"}

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	addresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s))

	addresses.TypedSpec().Addresses = []netaddr.IPPrefix{
		netaddr.MustParseIPPrefix("10.0.0.5/24"),
		netaddr.MustParseIPPrefix("192.168.1.1/24"),
		netaddr.MustParseIPPrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64"),
		netaddr.MustParseIPPrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7335/64"),
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, addresses))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			NodeIP, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodeIPType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := NodeIP.(*k8s.NodeIP).TypedSpec()

			suite.Assert().Equal("[10.0.0.5 2001:db8:85a3::8a2e:370:7334]", fmt.Sprintf("%s", spec.Addresses))

			return nil
		},
	))
}

func (suite *NodeIPSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestNodeIPSuite(t *testing.T) {
	suite.Run(t, new(NodeIPSuite))
}
