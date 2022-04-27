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

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/component-base/config/v1alpha1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

type KubeletSpecSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
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

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				kubeletSpec, err := suite.state.Get(
					suite.ctx,
					resource.NewMetadata(
						k8s.NamespaceName,
						k8s.KubeletSpecType,
						k8s.KubeletID,
						resource.VersionUndefined,
					),
				)
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
					}, spec.Args,
				)
				suite.Assert().Equal(cfg.TypedSpec().ExtraMounts, spec.ExtraMounts)

				suite.Assert().Equal([]interface{}{"10.96.0.10"}, spec.Config["clusterDNS"])
				suite.Assert().Equal("cluster.local", spec.Config["clusterDomain"])

				return nil
			},
		),
	)
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

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				kubeletSpec, err := suite.state.Get(
					suite.ctx,
					resource.NewMetadata(
						k8s.NamespaceName,
						k8s.KubeletSpecType,
						k8s.KubeletID,
						resource.VersionUndefined,
					),
				)
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
					}, spec.Args,
				)

				return nil
			},
		),
	)
}

func (suite *KubeletSpecSuite) TestReconcileWithExtraConfig() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v2.0.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.11"}
	cfg.TypedSpec().ClusterDomain = "some.local"
	cfg.TypedSpec().ExtraConfig = map[string]interface{}{
		"serverTLSBootstrap": true,
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "foo.com"

	suite.Require().NoError(suite.state.Create(suite.ctx, nodename))

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netaddr.IP{netaddr.MustParseIP("172.20.0.3")}

	suite.Require().NoError(suite.state.Create(suite.ctx, nodeIP))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				kubeletSpec, err := suite.state.Get(
					suite.ctx,
					resource.NewMetadata(
						k8s.NamespaceName,
						k8s.KubeletSpecType,
						k8s.KubeletID,
						resource.VersionUndefined,
					),
				)
				if err != nil {
					if state.IsNotFoundError(err) {
						return retry.ExpectedError(err)
					}

					return err
				}

				spec := kubeletSpec.(*k8s.KubeletSpec).TypedSpec()

				var kubeletConfiguration kubeletconfig.KubeletConfiguration

				if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(
					spec.Config,
					&kubeletConfiguration,
				); err != nil {
					return err
				}

				suite.Assert().Equal("/", kubeletConfiguration.CgroupRoot)
				suite.Assert().Equal(cfg.TypedSpec().ClusterDomain, kubeletConfiguration.ClusterDomain)
				suite.Assert().True(kubeletConfiguration.ServerTLSBootstrap)

				return nil
			},
		),
	)
}

func (suite *KubeletSpecSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKubeletSpecSuite(t *testing.T) {
	suite.Run(t, new(KubeletSpecSuite))
}

func TestNewKubeletConfigurationFail(t *testing.T) {
	for _, tt := range []struct {
		name        string
		extraConfig map[string]interface{}
		expectedErr string
	}{
		{
			name: "wrong fields",
			extraConfig: map[string]interface{}{
				"API":  "v1",
				"foo":  "bar",
				"Port": "xyz",
			},
			expectedErr: "error unmarshalling extra kubelet configuration: strict decoding error: unknown field \"API\", unknown field \"Port\", unknown field \"foo\"",
		},
		{
			name: "wrong field type",
			extraConfig: map[string]interface{}{
				"oomScoreAdj": "v1",
			},
			expectedErr: "error unmarshalling extra kubelet configuration: unrecognized type: int32",
		},
		{
			name: "not overridable",
			extraConfig: map[string]interface{}{
				"oomScoreAdj":    -300,
				"port":           81,
				"authentication": nil,
			},
			expectedErr: "2 errors occurred:\n\t* field \"authentication\" can't be overridden\n\t* field \"port\" can't be overridden\n\n",
		},
	} {
		tt := tt

		t.Run(
			tt.name, func(t *testing.T) {
				_, err := k8sctrl.NewKubeletConfiguration([]string{"10.96.0.10"}, "cluster.svc", tt.extraConfig)
				require.Error(t, err)

				assert.EqualError(t, err, tt.expectedErr)
			},
		)
	}
}

func TestNewKubeletConfigurationMerge(t *testing.T) {
	defaultKubeletConfig := kubeletconfig.KubeletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubeletconfig.SchemeGroupVersion.String(),
			Kind:       "KubeletConfiguration",
		},
		StaticPodPath: constants.ManifestsDirectory,
		Port:          constants.KubeletPort,
		Authentication: kubeletconfig.KubeletAuthentication{
			X509: kubeletconfig.KubeletX509Authentication{
				ClientCAFile: constants.KubernetesCACert,
			},
			Webhook: kubeletconfig.KubeletWebhookAuthentication{
				Enabled: pointer.ToBool(true),
			},
			Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
				Enabled: pointer.ToBool(false),
			},
		},
		Authorization: kubeletconfig.KubeletAuthorization{
			Mode: kubeletconfig.KubeletAuthorizationModeWebhook,
		},
		CgroupRoot:            "/",
		SystemCgroups:         constants.CgroupSystem,
		KubeletCgroups:        constants.CgroupKubelet,
		RotateCertificates:    true,
		ProtectKernelDefaults: true,
		Address:               "0.0.0.0",
		OOMScoreAdj:           pointer.ToInt32(constants.KubeletOOMScoreAdj),
		ClusterDomain:         "cluster.local",
		ClusterDNS:            []string{"10.0.0.5"},
		SerializeImagePulls:   pointer.ToBool(false),
		FailSwapOn:            pointer.ToBool(false),
		SystemReserved: map[string]string{
			"cpu":               constants.KubeletSystemReservedCPU,
			"memory":            constants.KubeletSystemReservedMemory,
			"pid":               constants.KubeletSystemReservedPid,
			"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
		},
		Logging: v1alpha1.LoggingConfiguration{
			Format: "json",
		},
		ShutdownGracePeriod:             metav1.Duration{Duration: constants.KubeletShutdownGracePeriod},
		ShutdownGracePeriodCriticalPods: metav1.Duration{Duration: constants.KubeletShutdownGracePeriodCriticalPods},
		StreamingConnectionIdleTimeout:  metav1.Duration{Duration: 5 * time.Minute},
		TLSMinVersion:                   "VersionTLS13",
	}

	for _, tt := range []struct {
		name              string
		extraConfig       map[string]interface{}
		expectedOverrides func(*kubeletconfig.KubeletConfiguration)
	}{
		{
			name: "override some",
			extraConfig: map[string]interface{}{
				"oomScoreAdj":             -300,
				"enableDebuggingHandlers": true,
			},
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.OOMScoreAdj = pointer.ToInt32(-300)
				kc.EnableDebuggingHandlers = pointer.ToBool(true)
			},
		},
		{
			name: "disable graceful shutdown",
			extraConfig: map[string]interface{}{
				"shutdownGracePeriod":             "0s",
				"shutdownGracePeriodCriticalPods": "0s",
			},
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.ShutdownGracePeriod = metav1.Duration{}
				kc.ShutdownGracePeriodCriticalPods = metav1.Duration{}
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			expected := defaultKubeletConfig
			tt.expectedOverrides(&expected)

			config, err := k8sctrl.NewKubeletConfiguration([]string{"10.0.0.5"}, "cluster.local", tt.extraConfig)

			require.NoError(t, err)

			assert.Equal(t, &expected, config)
		})
	}
}
