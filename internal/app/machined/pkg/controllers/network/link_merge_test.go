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

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/resources/network"
)

type LinkMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *LinkMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkMergeController{}))

	suite.startRuntime()
}

func (suite *LinkMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *LinkMergeSuite) assertLinks(requiredIDs []string, check func(*network.LinkSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.LinkSpecType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.LinkSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *LinkMergeSuite) assertNoLinks(id string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedError(fmt.Errorf("link %q is still there", id))
		}
	}

	return nil
}

func (suite *LinkMergeSuite) TestMerge() {
	loopback := network.NewLinkSpec(network.ConfigNamespaceName, "default/lo")
	*loopback.Status() = network.LinkSpecSpec{
		Name:        "lo",
		Up:          true,
		ConfigLayer: network.ConfigDefault,
	}

	dhcp := network.NewLinkSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp.Status() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1450,
		ConfigLayer: network.ConfigDHCP,
	}

	static := network.NewLinkSpec(network.ConfigNamespaceName, "configuration/eth0")
	*static.Status() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1500,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{loopback, dhcp, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertLinks([]string{
				"lo",
				"eth0",
			}, func(r *network.LinkSpec) error {
				switch r.Metadata().ID() {
				case "lo":
					suite.Assert().Equal(*loopback.Status(), *r.Status())
				case "eth0":
					suite.Assert().EqualValues(1500, r.Status().MTU) // static should override dhcp
				}

				return nil
			})
		}))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertLinks([]string{
				"lo",
				"eth0",
			}, func(r *network.LinkSpec) error {
				switch r.Metadata().ID() {
				case "lo":
					suite.Assert().Equal(*loopback.Status(), *r.Status())
				case "eth0":
					// reconcile happens eventually, so give it some time
					if r.Status().MTU != 1450 {
						return retry.ExpectedErrorf("MTU %d != 1450", r.Status().MTU)
					}
				}

				return nil
			})
		}))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, loopback.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoLinks("lo")
		}))
}

func TestLinkMergeSuite(t *testing.T) {
	suite.Run(t, new(LinkMergeSuite))
}
