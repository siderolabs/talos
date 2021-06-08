// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/resources/network"
)

type PlatformConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *PlatformConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)
}

func (suite *PlatformConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *PlatformConfigSuite) assertHostnames(requiredIDs []string, check func(*network.HostnameSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.HostnameSpecType, "", resource.VersionUndefined))
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

func (suite *PlatformConfigSuite) assertNoHostname(id string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.HostnameSpecType, "", resource.VersionUndefined))
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

func (suite *PlatformConfigSuite) TestNoPlatform() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoHostname("platform/hostname")
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMock() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl.c.talos-testbed.internal")},
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertHostnames([]string{
				"platform/hostname",
			}, func(r *network.HostnameSpec) error {
				suite.Assert().Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", r.TypedSpec().Hostname)
				suite.Assert().Equal("c.talos-testbed.internal", r.TypedSpec().Domainname)
				suite.Assert().Equal(network.ConfigPlatform, r.TypedSpec().ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockNoDomain() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl")},
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertHostnames([]string{
				"platform/hostname",
			}, func(r *network.HostnameSpec) error {
				suite.Assert().Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", r.TypedSpec().Hostname)
				suite.Assert().Equal("", r.TypedSpec().Domainname)
				suite.Assert().Equal(network.ConfigPlatform, r.TypedSpec().ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestPlatformConfigSuite(t *testing.T) {
	suite.Run(t, new(PlatformConfigSuite))
}

type platformMock struct {
	hostname []byte
}

func (mock *platformMock) Name() string {
	return "mock"
}

func (mock *platformMock) Configuration(context.Context) ([]byte, error) {
	return nil, nil
}

func (mock *platformMock) Hostname(context.Context) ([]byte, error) {
	return mock.hostname, nil
}

func (mock *platformMock) Mode() v1alpha1runtime.Mode {
	return v1alpha1runtime.ModeCloud
}

func (mock *platformMock) ExternalIPs(context.Context) ([]net.IP, error) {
	return nil, nil
}

func (mock *platformMock) KernelArgs() procfs.Parameters {
	return nil
}
