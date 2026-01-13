// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
package k8s_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/component-base/logs/api/v1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubeletSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *KubeletSpecSuite) TestReconcileDefault() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.29.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraArgs = map[string]k8s.ArgValues{"foo": {Values: []string{"bar"}}}
	cfg.TypedSpec().ExtraMounts = []specs.Mount{
		{
			Destination: "/tmp",
			Source:      "/var",
			Type:        "tmpfs",
		},
	}
	cfg.TypedSpec().CloudProviderExternal = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("172.20.0.2")}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodeIP))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodename))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(kubeletSpec *k8s.KubeletSpec, asrt *assert.Assertions) {
		spec := kubeletSpec.TypedSpec()

		asrt.Equal(cfg.TypedSpec().Image, spec.Image)
		asrt.Equal(
			[]string{
				"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubeconfig",
				"--cert-dir=/var/lib/kubelet/pki",
				"--cloud-provider=external",
				"--config=/etc/kubernetes/kubelet.yaml",
				"--foo=bar",
				"--hostname-override=example.com",
				"--kubeconfig=/etc/kubernetes/kubeconfig-kubelet",
				"--node-ip=172.20.0.2",
			}, spec.Args,
		)
		asrt.Equal(cfg.TypedSpec().ExtraMounts, spec.ExtraMounts)

		asrt.Equal([]any{"10.96.0.10"}, spec.Config["clusterDNS"])
		asrt.Equal("cluster.local", spec.Config["clusterDomain"])
	})
}

func (suite *KubeletSpecSuite) TestReconcileWithExplicitNodeIP() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.29.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraArgs = map[string]k8s.ArgValues{"node-ip": {Values: []string{"10.0.0.1"}}}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodename))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(kubeletSpec *k8s.KubeletSpec, asrt *assert.Assertions) {
		spec := kubeletSpec.TypedSpec()

		asrt.Equal(cfg.TypedSpec().Image, spec.Image)
		asrt.Equal(
			[]string{
				"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubeconfig",
				"--cert-dir=/var/lib/kubelet/pki",
				"--config=/etc/kubernetes/kubelet.yaml",
				"--hostname-override=example.com",
				"--kubeconfig=/etc/kubernetes/kubeconfig-kubelet",
				"--node-ip=10.0.0.1",
			}, spec.Args,
		)
	})
}

func (suite *KubeletSpecSuite) TestReconcileWithContainerRuntimeEndpointFlag() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.25.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraArgs = map[string]k8s.ArgValues{"node-ip": {Values: []string{"10.0.0.1"}}}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodename))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(kubeletSpec *k8s.KubeletSpec, asrt *assert.Assertions) {
		spec := kubeletSpec.TypedSpec()

		asrt.Equal(cfg.TypedSpec().Image, spec.Image)
		asrt.Equal(
			[]string{
				"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubeconfig",
				"--cert-dir=/var/lib/kubelet/pki",
				"--config=/etc/kubernetes/kubelet.yaml",
				"--container-runtime-endpoint=/run/containerd/containerd.sock",
				"--hostname-override=example.com",
				"--kubeconfig=/etc/kubernetes/kubeconfig-kubelet",
				"--node-ip=10.0.0.1",
			}, spec.Args,
		)

		var kubeletConfiguration kubeletconfig.KubeletConfiguration

		if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(
			spec.Config,
			&kubeletConfiguration,
		); err != nil {
			asrt.NoError(err)

			return
		}

		asrt.Empty(kubeletConfiguration.ContainerRuntimeEndpoint)
	})
}

func (suite *KubeletSpecSuite) TestReconcileWithExtraConfig() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v2.0.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.11"}
	cfg.TypedSpec().ClusterDomain = "some.local"
	cfg.TypedSpec().ExtraConfig = map[string]any{
		"serverTLSBootstrap": true,
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "foo.com"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodename))

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("172.20.0.3")}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodeIP))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(kubeletSpec *k8s.KubeletSpec, asrt *assert.Assertions) {
		spec := kubeletSpec.TypedSpec()

		var kubeletConfiguration kubeletconfig.KubeletConfiguration

		if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(
			spec.Config,
			&kubeletConfiguration,
		); err != nil {
			asrt.NoError(err)

			return
		}

		asrt.Equal("/", kubeletConfiguration.CgroupRoot)
		asrt.Equal(cfg.TypedSpec().ClusterDomain, kubeletConfiguration.ClusterDomain)
		asrt.True(kubeletConfiguration.ServerTLSBootstrap)
	})
}

func (suite *KubeletSpecSuite) TestReconcileWithSkipNodeRegistration() {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v2.0.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.11"}
	cfg.TypedSpec().ClusterDomain = "some.local"
	cfg.TypedSpec().SkipNodeRegistration = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "foo.com"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodename))

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("172.20.0.3")}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodeIP))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(kubeletSpec *k8s.KubeletSpec, asrt *assert.Assertions) {
		spec := kubeletSpec.TypedSpec()

		var kubeletConfiguration kubeletconfig.KubeletConfiguration

		if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(
			spec.Config,
			&kubeletConfiguration,
		); err != nil {
			asrt.NoError(err)

			return
		}

		asrt.Equal("/", kubeletConfiguration.CgroupRoot)
		asrt.Equal(cfg.TypedSpec().ClusterDomain, kubeletConfiguration.ClusterDomain)
		asrt.Equal([]string{
			"--cert-dir=/var/lib/kubelet/pki",
			"--config=/etc/kubernetes/kubelet.yaml",
			"--hostname-override=foo.com",
			"--node-ip=172.20.0.3",
		}, spec.Args)
	})
}

func TestKubeletSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KubeletSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.KubeletSpecController{}))
			},
		},
	})
}

func TestNewKubeletConfigurationFail(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		cfgSpec     *k8s.KubeletConfigSpec
		expectedErr string
	}{
		{
			name: "wrong fields",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:    []string{"10.96.0.10"},
				ClusterDomain: "cluster.svc",
				ExtraConfig: map[string]any{
					"API":  "v1",
					"foo":  "bar",
					"Port": "xyz",
				},
			},
			expectedErr: "error unmarshalling extra kubelet configuration: strict decoding error: unknown field \"API\", unknown field \"Port\", unknown field \"foo\"",
		},
		{
			name: "wrong field type",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:    []string{"10.96.0.10"},
				ClusterDomain: "cluster.svc",
				ExtraConfig: map[string]any{
					"oomScoreAdj": "v1",
				},
			},
			expectedErr: "error unmarshalling extra kubelet configuration: unrecognized type: int32",
		},
		{
			name: "not overridable",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:    []string{"10.96.0.10"},
				ClusterDomain: "cluster.svc",
				ExtraConfig: map[string]any{
					"oomScoreAdj":    -300,
					"port":           81,
					"authentication": nil,
				},
			},
			expectedErr: "2 errors occurred:\n\t* field \"authentication\" can't be overridden\n\t* field \"port\" can't be overridden\n\n",
		},
	} {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := k8sctrl.NewKubeletConfiguration(tt.cfgSpec, compatibility.VersionFromImageRef(""), machine.TypeWorker)
				require.Error(t, err)

				assert.EqualError(t, err, tt.expectedErr)
			},
		)
	}
}

func TestNewKubeletConfigurationMerge(t *testing.T) {
	t.Parallel()

	defaultKubeletConfig := kubeletconfig.KubeletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubeletconfig.SchemeGroupVersion.String(),
			Kind:       "KubeletConfiguration",
		},
		Port: constants.KubeletPort,
		Authentication: kubeletconfig.KubeletAuthentication{
			X509: kubeletconfig.KubeletX509Authentication{
				ClientCAFile: constants.KubernetesCACert,
			},
			Webhook: kubeletconfig.KubeletWebhookAuthentication{
				Enabled: pointer.To(true),
			},
			Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
				Enabled: pointer.To(false),
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
		OOMScoreAdj:           pointer.To[int32](constants.KubeletOOMScoreAdj),
		ClusterDomain:         "cluster.local",
		ClusterDNS:            []string{"10.0.0.5"},
		SerializeImagePulls:   pointer.To(false),
		FailSwapOn:            pointer.To(false),
		SystemReserved: map[string]string{
			"cpu":               constants.KubeletSystemReservedCPU,
			"memory":            constants.KubeletSystemReservedMemoryWorker,
			"pid":               constants.KubeletSystemReservedPid,
			"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
		},
		Logging: v1.LoggingConfiguration{
			Format: "json",
		},
		ShutdownGracePeriod:             metav1.Duration{Duration: constants.KubeletShutdownGracePeriod},
		ShutdownGracePeriodCriticalPods: metav1.Duration{Duration: constants.KubeletShutdownGracePeriodCriticalPods},
		StreamingConnectionIdleTimeout:  metav1.Duration{Duration: 5 * time.Minute},
		TLSMinVersion:                   "VersionTLS13",
		StaticPodPath:                   constants.ManifestsDirectory,
		ContainerRuntimeEndpoint:        "unix://" + constants.CRIContainerdAddress,
		ResolverConfig:                  pointer.To(constants.PodResolvConfPath),
	}

	for _, tt := range []struct {
		name              string
		cfgSpec           *k8s.KubeletConfigSpec
		kubeletVersion    compatibility.Version
		expectedOverrides func(*kubeletconfig.KubeletConfiguration)
		machineType       machine.Type
	}{
		{
			name: "override some",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:    []string{"10.0.0.5"},
				ClusterDomain: "cluster.local",
				ExtraConfig: map[string]any{
					"oomScoreAdj":             -300,
					"enableDebuggingHandlers": true,
				},
			},
			kubeletVersion: compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.29.0"),
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.OOMScoreAdj = pointer.To[int32](-300)
				kc.EnableDebuggingHandlers = pointer.To(true)
			},
			machineType: machine.TypeWorker,
		},
		{
			name: "controlplane",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:    []string{"10.0.0.5"},
				ClusterDomain: "cluster.local",
			},
			kubeletVersion: compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.29.0"),
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.SystemReserved["memory"] = constants.KubeletSystemReservedMemoryControlPlane
				kc.RegisterWithTaints = []corev1.Taint{
					{
						Key:    constants.LabelNodeRoleControlPlane,
						Effect: corev1.TaintEffectNoSchedule,
					},
				}
			},
			machineType: machine.TypeControlPlane,
		},
		{
			name: "disable graceful shutdown",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:    []string{"10.0.0.5"},
				ClusterDomain: "cluster.local",
				ExtraConfig: map[string]any{
					"shutdownGracePeriod":             "0s",
					"shutdownGracePeriodCriticalPods": "0s",
				},
			},
			kubeletVersion: compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.29.0"),
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.ShutdownGracePeriod = metav1.Duration{}
				kc.ShutdownGracePeriodCriticalPods = metav1.Duration{}
			},
			machineType: machine.TypeWorker,
		},
		{
			name: "enable seccomp default",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:                   []string{"10.0.0.5"},
				ClusterDomain:                "cluster.local",
				DefaultRuntimeSeccompEnabled: true,
			},
			kubeletVersion: compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.29.0"),
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.SeccompDefault = pointer.To(true)
			},
			machineType: machine.TypeWorker,
		},
		{
			name: "enable skipNodeRegistration",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:           []string{"10.0.0.5"},
				ClusterDomain:        "cluster.local",
				SkipNodeRegistration: true,
			},
			kubeletVersion: compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.29.0"),
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.Authentication.Webhook.Enabled = pointer.To(false)
				kc.Authorization.Mode = kubeletconfig.KubeletAuthorizationModeAlwaysAllow
			},
			machineType: machine.TypeWorker,
		},
		{
			name: "disable manifests directory",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:                []string{"10.0.0.5"},
				ClusterDomain:             "cluster.local",
				DisableManifestsDirectory: true,
			},
			kubeletVersion: compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.29.0"),
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.StaticPodPath = ""
			},
			machineType: machine.TypeWorker,
		},
		{
			name: "enable local FS quota monitoring",
			cfgSpec: &k8s.KubeletConfigSpec{
				ClusterDNS:              []string{"10.0.0.5"},
				ClusterDomain:           "cluster.local",
				EnableFSQuotaMonitoring: true,
			},
			kubeletVersion: compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.29.0"),
			expectedOverrides: func(kc *kubeletconfig.KubeletConfiguration) {
				kc.FeatureGates = map[string]bool{
					"LocalStorageCapacityIsolationFSQuotaMonitoring": true,
				}
			},
			machineType: machine.TypeWorker,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected := defaultKubeletConfig.DeepCopy()
			tt.expectedOverrides(expected)

			config, err := k8sctrl.NewKubeletConfiguration(tt.cfgSpec, tt.kubeletVersion, tt.machineType)

			require.NoError(t, err)

			assert.Equal(t, expected, config)
		})
	}
}
