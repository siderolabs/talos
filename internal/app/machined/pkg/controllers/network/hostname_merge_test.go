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
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type HostnameMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *HostnameMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HostnameMergeController{}))

	suite.startRuntime()
}

func (suite *HostnameMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *HostnameMergeSuite) assertHostnames(requiredIDs []string, check func(*network.HostnameSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.HostnameSpecType, "", resource.VersionUndefined),
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

		if err = check(res.(*network.HostnameSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *HostnameMergeSuite) TestMerge() {
	def := network.NewHostnameSpec(network.ConfigNamespaceName, "default/hostname")
	*def.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "foo",
		Domainname:  "tld",
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewHostnameSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "eth-0",
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewHostnameSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "eth-1",
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewHostnameSpec(network.ConfigNamespaceName, "configuration/hostname")
	*static.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "bar",
		Domainname:  "com",
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHostnames(
					[]string{
						"hostname",
					}, func(r *network.HostnameSpec) error {
						suite.Assert().Equal("bar.com", r.TypedSpec().FQDN())
						suite.Assert().Equal("bar", r.TypedSpec().Hostname)
						suite.Assert().Equal("com", r.TypedSpec().Domainname)

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
				return suite.assertHostnames(
					[]string{
						"hostname",
					}, func(r *network.HostnameSpec) error {
						if r.TypedSpec().FQDN() != "eth-0" {
							return retry.ExpectedErrorf("unexpected hostname %q", r.TypedSpec().FQDN())
						}

						return nil
					},
				)
			},
		),
	)
}

func (suite *HostnameMergeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewHostnameSpec(network.ConfigNamespaceName, "bar"),
		),
	)
}

func TestHostnameMergeSuite(t *testing.T) {
	suite.Run(t, new(HostnameMergeSuite))
}
