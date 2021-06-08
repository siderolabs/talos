// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"sort"
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

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/resources/network"
)

type NodeAddressSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *NodeAddressSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressStatusController{}))
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkStatusController{}))
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

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, "", resource.VersionUndefined))
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
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertAddresses([]string{
				network.NodeAddressDefaultID,
				network.NodeAddressCurrentID,
				network.NodeAddressAccumulativeID,
			}, func(r *network.NodeAddress) error {
				addrs := r.TypedSpec().Addresses

				suite.T().Logf("id %q val %s", r.Metadata().ID(), addrs)

				suite.Assert().True(sort.SliceIsSorted(addrs, func(i, j int) bool {
					return addrs[i].Compare(addrs[j]) < 0
				}), "addresses %s", addrs)

				if r.Metadata().ID() == network.NodeAddressDefaultID {
					suite.Assert().Len(addrs, 1)
				} else {
					suite.Assert().GreaterOrEqual(len(addrs), 1)
				}

				return nil
			})
		}))
}

func (suite *NodeAddressSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewAddressStatus(network.NamespaceName, "bar")))
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewLinkStatus(network.NamespaceName, "bar")))
}

func TestNodeAddressSuite(t *testing.T) {
	suite.Run(t, new(NodeAddressSuite))
}
