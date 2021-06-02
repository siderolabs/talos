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

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/network"
)

type TimeServerMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *TimeServerMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.TimeServerMergeController{}))

	suite.startRuntime()
}

func (suite *TimeServerMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *TimeServerMergeSuite) assertTimeServers(requiredIDs []string, check func(*network.TimeServerSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.NamespaceName, network.TimeServerSpecType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.TimeServerSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *TimeServerMergeSuite) TestMerge() {
	def := network.NewTimeServerSpec(network.ConfigNamespaceName, "default/timeservers")
	*def.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{constants.DefaultNTPServer},
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewTimeServerSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"ntp.eth0"},
		ConfigLayer: network.ConfigDHCP,
	}

	dhcp2 := network.NewTimeServerSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"ntp.eth1"},
		ConfigLayer: network.ConfigDHCP,
	}

	static := network.NewTimeServerSpec(network.ConfigNamespaceName, "configuration/timeservers")
	*static.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"my.ntp"},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeServers([]string{
				"timeservers",
			}, func(r *network.TimeServerSpec) error {
				suite.Assert().Equal(*static.TypedSpec(), *r.TypedSpec())

				return nil
			})
		}))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeServers([]string{
				"timeservers",
			}, func(r *network.TimeServerSpec) error {
				if !reflect.DeepEqual(r.TypedSpec().NTPServers, []string{"ntp.eth0", "ntp.eth1"}) {
					return retry.ExpectedErrorf("unexpected servers %q", r.TypedSpec().NTPServers)
				}

				return nil
			})
		}))
}

func TestTimeServerMergeSuite(t *testing.T) {
	suite.Run(t, new(TimeServerMergeSuite))
}
