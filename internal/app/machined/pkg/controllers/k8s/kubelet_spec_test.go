// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

type KubeletSpecSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *KubeletSpecSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.KubeletSpecController{}))

	suite.startRuntime()
}

func (suite *KubeletSpecSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *KubeletSpecSuite) TestReconcileDefault() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.0.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraArgs = map[string]string{"foo": "bar"}
	cfg.TypedSpec().ExtraMounts = []specs.Mount{
		{
			Destination: "/tmp",
			Source:      "/var",
			Type:        "tmpfs",
		},
	}
	cfg.TypedSpec().CloudProviderExternal = true

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netaddr.IP{netaddr.MustParseIP("172.20.0.2")}

	suite.Require().NoError(suite.state.Create(suite.ctx, nodeIP))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"

	suite.Require().NoError(suite.state.Create(suite.ctx, nodename))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			kubeletSpec, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := kubeletSpec.(*k8s.KubeletSpec).TypedSpec()

			suite.Assert().Equal(cfg.TypedSpec().Image, spec.Image)
			suite.Assert().Equal(
				[]string{
					"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubeconfig",
					"--cert-dir=/var/lib/kubelet/pki",
					"--cloud-provider=external",
					"--config=/etc/kubernetes/kubelet.yaml",
					"--container-runtime=remote",
					"--container-runtime-endpoint=unix:///run/containerd/containerd.sock",
					"--foo=bar",
					"--hostname-override=example.com",
					"--kubeconfig=/etc/kubernetes/kubeconfig-kubelet",
					"--node-ip=172.20.0.2",
				}, spec.Args)
			suite.Assert().Equal(cfg.TypedSpec().ExtraMounts, spec.ExtraMounts)

			suite.Assert().Equal([]interface{}{"10.96.0.10"}, spec.Config["clusterDNS"])
			suite.Assert().Equal("cluster.local", spec.Config["clusterDomain"])

			return nil
		},
	))
}

func (suite *KubeletSpecSuite) TestReconcileWithExplicitNodeIP() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.0.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraArgs = map[string]string{"node-ip": "10.0.0.1"}

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"

	suite.Require().NoError(suite.state.Create(suite.ctx, nodename))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			kubeletSpec, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := kubeletSpec.(*k8s.KubeletSpec).TypedSpec()

			suite.Assert().Equal(cfg.TypedSpec().Image, spec.Image)
			suite.Assert().Equal(
				[]string{
					"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubeconfig",
					"--cert-dir=/var/lib/kubelet/pki",
					"--config=/etc/kubernetes/kubelet.yaml",
					"--container-runtime=remote",
					"--container-runtime-endpoint=unix:///run/containerd/containerd.sock",
					"--hostname-override=example.com",
					"--kubeconfig=/etc/kubernetes/kubeconfig-kubelet",
					"--node-ip=10.0.0.1",
				}, spec.Args)

			return nil
		},
	))
}

func (suite *KubeletSpecSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKubeletSpecSuite(t *testing.T) {
	suite.Run(t, new(KubeletSpecSuite))
}
