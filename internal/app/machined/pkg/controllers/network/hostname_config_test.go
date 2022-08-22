// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type HostnameConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *HostnameConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)
}

func (suite *HostnameConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *HostnameConfigSuite) assertHostnames(requiredIDs []string, check func(*network.HostnameSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.ConfigNamespaceName, network.HostnameSpecType, "", resource.VersionUndefined),
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

func (suite *HostnameConfigSuite) assertNoHostname(id string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.ConfigNamespaceName, network.HostnameSpecType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedError(fmt.Errorf("spec %q is still there", id))
		}
	}

	return nil
}

func (suite *HostnameConfigSuite) TestNoDefaultWithoutMachineConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HostnameConfigController{}))

	suite.startRuntime()

	defaultAddress := network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID)
	defaultAddress.TypedSpec().Addresses = []netaddr.IPPrefix{netaddr.MustParseIPPrefix("33.11.22.44/32")}

	suite.Require().NoError(suite.state.Create(suite.ctx, defaultAddress))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHostnames(nil, func(r *network.HostnameSpec) error {
					suite.Assert().NotEqual("default/hostname", r.Metadata().ID(), "default hostname is still there")

					return nil
				},
				)
			},
		),
	)
}

func (suite *HostnameConfigSuite) TestDefaultIPBasedHostname() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HostnameConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{ConfigVersion: "v1alpha1"})
	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	defaultAddress := network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID)
	defaultAddress.TypedSpec().Addresses = []netaddr.IPPrefix{netaddr.MustParseIPPrefix("33.11.22.44/32")}

	suite.Require().NoError(suite.state.Create(suite.ctx, defaultAddress))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHostnames(
					[]string{
						"default/hostname",
					}, func(r *network.HostnameSpec) error {
						suite.Assert().Equal("talos-33-11-22-44", r.TypedSpec().Hostname)
						suite.Assert().Equal("", r.TypedSpec().Domainname)
						suite.Assert().Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)

						return nil
					},
				)
			},
		),
	)
}

func (suite *HostnameConfigSuite) TestDefaultStableHostname() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HostnameConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineFeatures: &v1alpha1.FeaturesConfig{
					StableHostname: pointer.To(true),
				},
			},
		},
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	id := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	id.TypedSpec().NodeID = "fGdOI05hVrx3YMagLo0Bwxa2Nm9BAswWm8XLeEj0aS4"

	suite.Require().NoError(suite.state.Create(suite.ctx, id))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHostnames(
					[]string{
						"default/hostname",
					}, func(r *network.HostnameSpec) error {
						suite.Assert().Equal("talos-hwz-sw5", r.TypedSpec().Hostname)

						return nil
					},
				)
			},
		),
	)
}

func (suite *HostnameConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.HostnameConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2:172.21.0.1:172.20.0.1:255.255.255.0:master1.domain.tld:eth1::10.0.0.1:10.0.0.2:10.0.0.1"),
			},
		),
	)

	suite.startRuntime()

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHostnames(
					[]string{
						"cmdline/hostname",
					}, func(r *network.HostnameSpec) error {
						suite.Assert().Equal("master1", r.TypedSpec().Hostname)
						suite.Assert().Equal("domain.tld", r.TypedSpec().Domainname)
						suite.Assert().Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)

						return nil
					},
				)
			},
		),
	)
}

func (suite *HostnameConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HostnameConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineNetwork: &v1alpha1.NetworkConfig{
					NetworkHostname: "foo",
				},
			},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{
					Endpoint: &v1alpha1.Endpoint{
						URL: u,
					},
				},
			},
		},
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHostnames(
					[]string{
						"configuration/hostname",
					}, func(r *network.HostnameSpec) error {
						suite.Assert().Equal("foo", r.TypedSpec().Hostname)
						suite.Assert().Equal("", r.TypedSpec().Domainname)
						suite.Assert().Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

						return nil
					},
				)
			},
		),
	)

	_, err = suite.state.UpdateWithConflicts(
		suite.ctx, cfg.Metadata(), func(r resource.Resource) error {
			r.(*config.MachineConfig).Config().(*v1alpha1.Config).MachineConfig.MachineNetwork.NetworkHostname = strings.Repeat(
				"a",
				128,
			)

			return nil
		},
	)
	suite.Require().NoError(err)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoHostname("configuration/hostname")
			},
		),
	)
}

func (suite *HostnameConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(
		context.Background(), config.NewMachineConfig(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
			},
		),
	)
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Require().NoError(err)

	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewNodeAddress(network.NamespaceName, "bar"),
		),
	)
}

func TestHostnameConfigSuite(t *testing.T) {
	suite.Run(t, new(HostnameConfigSuite))
}
