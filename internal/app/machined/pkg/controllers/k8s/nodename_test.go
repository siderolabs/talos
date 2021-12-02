// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"fmt"
	"log"
	"net/url"
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

	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type NodenameSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *NodenameSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.NodenameController{}))

	suite.startRuntime()
}

func (suite *NodenameSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

//nolint:dupl
func (suite *NodenameSuite) assertNodename(expected string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	if len(resources.Items) != 1 {
		return retry.ExpectedErrorf("expected 1 item, got %d", len(resources.Items))
	}

	if resources.Items[0].Metadata().ID() != k8s.NodenameID {
		return fmt.Errorf("unexpected ID")
	}

	if resources.Items[0].(*k8s.Nodename).TypedSpec().Nodename != expected {
		return retry.ExpectedErrorf("expected %q, got %q", expected, resources.Items[0].(*k8s.Nodename).TypedSpec().Nodename)
	}

	return nil
}

func (suite *NodenameSuite) TestDefault() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: u,
				},
			},
		},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "foo"
	hostnameStatus.TypedSpec().Domainname = "bar.ltd"

	suite.Require().NoError(suite.state.Create(suite.ctx, hostnameStatus))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNodename("foo")
		},
	))
}

func (suite *NodenameSuite) TestFQDN() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineKubelet: &v1alpha1.KubeletConfig{
				KubeletRegisterWithFQDN: true,
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: u,
				},
			},
		},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "foo"
	hostnameStatus.TypedSpec().Domainname = "bar.ltd"

	suite.Require().NoError(suite.state.Create(suite.ctx, hostnameStatus))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNodename("foo.bar.ltd")
		},
	))
}

func (suite *NodenameSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(context.Background(), config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
	}))
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Require().NoError(err)

	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewHostnameStatus(network.NamespaceName, "bar")))
}

func TestNodenameSuite(t *testing.T) {
	suite.Run(t, new(NodenameSuite))
}
