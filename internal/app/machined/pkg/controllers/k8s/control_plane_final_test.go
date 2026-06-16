// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type ControlPlaneSchedulerFinalSuite struct {
	ctest.DefaultSuite
}

//nolint:dupl
func (suite *ControlPlaneSchedulerFinalSuite) TestTransform() {
	kubeconfigPath := filepath.Join(constants.KubernetesSchedulerSecretsDir, "kubeconfig")
	configPath := filepath.Join(constants.KubernetesSchedulerConfigDir, "scheduler-config.yaml")

	defaultArgs := []string{
		"/usr/local/bin/kube-scheduler",
		"--authentication-kubeconfig=" + kubeconfigPath,
		"--authentication-tolerate-lookup-failure=false",
		"--authorization-kubeconfig=" + kubeconfigPath,
		"--bind-address=127.0.0.1",
		"--config=" + configPath,
		"--leader-elect=true",
		"--profiling=false",
		"--tls-min-version=VersionTLS13",
	}

	for _, tt := range []struct {
		name     string
		input    k8s.SchedulerConfigSpec
		expected k8s.SchedulerConfigSpec
	}{
		{
			name: "disabled drops everything but Enabled",
			input: k8s.SchedulerConfigSpec{
				Enabled: false,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				Config:  map[string]any{"percentageOfNodesToScore": int64(50)},
			},
			expected: k8s.SchedulerConfigSpec{
				Enabled: false,
			},
		},
		{
			name: "enabled with empty config gets defaults injected",
			input: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				Config:  map[string]any{},
			},
			expected: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				Args:    defaultArgs,
				Config: map[string]any{
					"apiVersion": "kubescheduler.config.k8s.io/v1",
					"kind":       "KubeSchedulerConfiguration",
					"clientConnection": map[string]any{
						"kubeconfig": kubeconfigPath,
					},
				},
			},
		},
		{
			name: "extra args override defaults and add new",
			input: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				ExtraArgs: map[string]k8s.ArgValues{
					"bind-address": {Values: []string{"0.0.0.0"}},
					"v":            {Values: []string{"4"}},
				},
				Config: map[string]any{},
			},
			expected: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				Args: []string{
					"/usr/local/bin/kube-scheduler",
					"--authentication-kubeconfig=" + kubeconfigPath,
					"--authentication-tolerate-lookup-failure=false",
					"--authorization-kubeconfig=" + kubeconfigPath,
					"--bind-address=0.0.0.0",
					"--config=" + configPath,
					"--leader-elect=true",
					"--profiling=false",
					"--tls-min-version=VersionTLS13",
					"--v=4",
				},
				Config: map[string]any{
					"apiVersion": "kubescheduler.config.k8s.io/v1",
					"kind":       "KubeSchedulerConfiguration",
					"clientConnection": map[string]any{
						"kubeconfig": kubeconfigPath,
					},
				},
			},
		},
		{
			// Regression for siderolabs/talos#13445: the YAML parser emits Go
			// ints for integers, and runtime.DeepCopyJSON panics on those.
			name: "go int values in config are normalized to int64",
			input: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				Config: map[string]any{
					"profiles": []any{
						map[string]any{
							"schedulerName": "default-scheduler",
							"pluginConfig": []any{
								map[string]any{
									"name": "PodTopologySpread",
									"args": map[string]any{
										"defaultingType": "List",
										"defaultConstraints": []any{
											map[string]any{
												"maxSkew":           int(1),
												"topologyKey":       "kubernetes.io/hostname",
												"whenUnsatisfiable": "ScheduleAnyway",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				Args:    defaultArgs,
				Config: map[string]any{
					"apiVersion": "kubescheduler.config.k8s.io/v1",
					"kind":       "KubeSchedulerConfiguration",
					"clientConnection": map[string]any{
						"kubeconfig": kubeconfigPath,
					},
					"profiles": []any{
						map[string]any{
							"schedulerName": "default-scheduler",
							"pluginConfig": []any{
								map[string]any{
									"name": "PodTopologySpread",
									"args": map[string]any{
										"defaultingType": "List",
										"defaultConstraints": []any{
											map[string]any{
												"maxSkew":           int64(1),
												"topologyKey":       "kubernetes.io/hostname",
												"whenUnsatisfiable": "ScheduleAnyway",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "pass-through fields and config preserved alongside injected keys",
			input: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				ExtraVolumes: []k8s.ExtraVolume{
					{Name: "foo", HostPath: "/var/foo", MountPath: "/foo", ReadOnly: true},
				},
				EnvironmentVariables: map[string]string{
					"GOMAXPROCS": "4",
				},
				Resources: k8s.Resources{
					Requests: map[string]string{"cpu": "150m", "memory": "2Gi"},
					Limits:   map[string]string{"cpu": "300m", "memory": "4Gi"},
				},
				Config: map[string]any{
					"percentageOfNodesToScore": int64(50),
					"clientConnection": map[string]any{
						"qps":   float64(100),
						"burst": int64(200),
					},
					"profiles": []any{
						map[string]any{
							"schedulerName": "talos-scheduler",
						},
					},
				},
			},
			expected: k8s.SchedulerConfigSpec{
				Enabled: true,
				Image:   "registry.k8s.io/kube-scheduler:v1.32.0",
				ExtraVolumes: []k8s.ExtraVolume{
					{Name: "foo", HostPath: "/var/foo", MountPath: "/foo", ReadOnly: true},
				},
				EnvironmentVariables: map[string]string{
					"GOMAXPROCS": "4",
				},
				Resources: k8s.Resources{
					Requests: map[string]string{"cpu": "150m", "memory": "2Gi"},
					Limits:   map[string]string{"cpu": "300m", "memory": "4Gi"},
				},
				Args: defaultArgs,
				Config: map[string]any{
					"apiVersion":               "kubescheduler.config.k8s.io/v1",
					"kind":                     "KubeSchedulerConfiguration",
					"percentageOfNodesToScore": int64(50),
					"clientConnection": map[string]any{
						"kubeconfig": kubeconfigPath,
						"qps":        float64(100),
						"burst":      int64(200),
					},
					"profiles": []any{
						map[string]any{
							"schedulerName": "talos-scheduler",
						},
					},
				},
			},
		},
	} {
		suite.Run(tt.name, func() {
			in := k8s.NewSchedulerConfig(k8s.SchedulerConfigID)
			*in.TypedSpec() = tt.input

			if _, err := suite.State().Get(suite.Ctx(), in.Metadata()); err == nil {
				ctest.UpdateWithConflicts(suite, in, func(r *k8s.SchedulerConfig) error {
					*r.TypedSpec() = tt.input

					return nil
				})
			} else {
				suite.Create(in)
			}

			ctest.AssertResource(suite, k8s.FinalSchedulerConfigID, func(r *k8s.SchedulerConfig, asrt *assert.Assertions) {
				asrt.Equal(tt.expected, *r.TypedSpec())
			})
		})
	}
}

func TestControlPlaneSchedulerFinalSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ControlPlaneSchedulerFinalSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneSchedulerFinalController()))
			},
		},
	})
}

type ControlPlaneControllerManagerFinalSuite struct {
	ctest.DefaultSuite
}

//nolint:dupl
func (suite *ControlPlaneControllerManagerFinalSuite) TestTransform() {
	secretsDir := constants.KubernetesControllerManagerSecretsDir
	kubeconfigPath := filepath.Join(secretsDir, "kubeconfig")
	caCrtPath := filepath.Join(secretsDir, "ca.crt")
	caKeyPath := filepath.Join(secretsDir, "ca.key")
	saKeyPath := filepath.Join(secretsDir, "service-account.key")

	// k8s < 1.33 still accepts --cloud-provider; k8s >= 1.33 has it removed.
	imagePreRemoval := "registry.k8s.io/kube-controller-manager:v1.32.0"
	imagePostRemoval := "registry.k8s.io/kube-controller-manager:v1.33.0"

	for _, tt := range []struct {
		name     string
		input    k8s.ControllerManagerConfigSpec
		expected k8s.ControllerManagerConfigSpec
	}{
		{
			name: "disabled drops everything but Enabled",
			input: k8s.ControllerManagerConfigSpec{
				Enabled:      false,
				Image:        imagePreRemoval,
				PodCIDRs:     []string{"10.244.0.0/16"},
				ServiceCIDRs: []string{"10.96.0.0/12"},
			},
			expected: k8s.ControllerManagerConfigSpec{
				Enabled: false,
			},
		},
		{
			name: "enabled produces canonical args",
			input: k8s.ControllerManagerConfigSpec{
				Enabled:      true,
				Image:        imagePostRemoval,
				PodCIDRs:     []string{"10.244.0.0/16"},
				ServiceCIDRs: []string{"10.96.0.0/12"},
			},
			expected: k8s.ControllerManagerConfigSpec{
				Enabled: true,
				Image:   imagePostRemoval,
				Args: []string{
					"/usr/local/bin/kube-controller-manager",
					"--use-service-account-credentials",
					"--allocate-node-cidrs=true",
					"--authentication-kubeconfig=" + kubeconfigPath,
					"--authorization-kubeconfig=" + kubeconfigPath,
					"--bind-address=127.0.0.1",
					"--cluster-cidr=10.244.0.0/16",
					"--cluster-signing-cert-file=" + caCrtPath,
					"--cluster-signing-key-file=" + caKeyPath,
					"--configure-cloud-routes=false",
					"--controllers=*",
					"--kubeconfig=" + kubeconfigPath,
					"--leader-elect=true",
					"--profiling=false",
					"--root-ca-file=" + caCrtPath,
					"--service-account-private-key-file=" + saKeyPath,
					"--service-cluster-ip-range=10.96.0.0/12",
					"--terminated-pod-gc-threshold=100",
					"--tls-min-version=VersionTLS13",
					"--use-service-account-credentials=true",
				},
			},
		},
		{
			name: "cloud provider keeps --cloud-provider on k8s < 1.33",
			input: k8s.ControllerManagerConfigSpec{
				Enabled:       true,
				Image:         imagePreRemoval,
				CloudProvider: "external",
				PodCIDRs:      []string{"10.244.0.0/16"},
				ServiceCIDRs:  []string{"10.96.0.0/12"},
			},
			expected: k8s.ControllerManagerConfigSpec{
				Enabled: true,
				Image:   imagePreRemoval,
				Args: []string{
					"/usr/local/bin/kube-controller-manager",
					"--use-service-account-credentials",
					"--allocate-node-cidrs=true",
					"--authentication-kubeconfig=" + kubeconfigPath,
					"--authorization-kubeconfig=" + kubeconfigPath,
					"--bind-address=127.0.0.1",
					"--cloud-provider=external",
					"--cluster-cidr=10.244.0.0/16",
					"--cluster-signing-cert-file=" + caCrtPath,
					"--cluster-signing-key-file=" + caKeyPath,
					"--configure-cloud-routes=false",
					"--controllers=*",
					"--kubeconfig=" + kubeconfigPath,
					"--leader-elect=true",
					"--profiling=false",
					"--root-ca-file=" + caCrtPath,
					"--service-account-private-key-file=" + saKeyPath,
					"--service-cluster-ip-range=10.96.0.0/12",
					"--terminated-pod-gc-threshold=100",
					"--tls-min-version=VersionTLS13",
					"--use-service-account-credentials=true",
				},
			},
		},
		{
			name: "cloud provider dropped on k8s >= 1.33",
			input: k8s.ControllerManagerConfigSpec{
				Enabled:       true,
				Image:         imagePostRemoval,
				CloudProvider: "external",
				PodCIDRs:      []string{"10.244.0.0/16"},
				ServiceCIDRs:  []string{"10.96.0.0/12"},
			},
			expected: k8s.ControllerManagerConfigSpec{
				Enabled: true,
				Image:   imagePostRemoval,
				Args: []string{
					"/usr/local/bin/kube-controller-manager",
					"--use-service-account-credentials",
					"--allocate-node-cidrs=true",
					"--authentication-kubeconfig=" + kubeconfigPath,
					"--authorization-kubeconfig=" + kubeconfigPath,
					"--bind-address=127.0.0.1",
					"--cluster-cidr=10.244.0.0/16",
					"--cluster-signing-cert-file=" + caCrtPath,
					"--cluster-signing-key-file=" + caKeyPath,
					"--configure-cloud-routes=false",
					"--controllers=*",
					"--kubeconfig=" + kubeconfigPath,
					"--leader-elect=true",
					"--profiling=false",
					"--root-ca-file=" + caCrtPath,
					"--service-account-private-key-file=" + saKeyPath,
					"--service-cluster-ip-range=10.96.0.0/12",
					"--terminated-pod-gc-threshold=100",
					"--tls-min-version=VersionTLS13",
					"--use-service-account-credentials=true",
				},
			},
		},
		{
			name: "multiple pod and service CIDRs joined; extra controllers merged additively",
			input: k8s.ControllerManagerConfigSpec{
				Enabled:      true,
				Image:        imagePostRemoval,
				PodCIDRs:     []string{"10.244.0.0/16", "fd00::/64"},
				ServiceCIDRs: []string{"10.96.0.0/12", "fd00:1::/108"},
				ExtraArgs: map[string]k8s.ArgValues{
					"controllers": {Values: []string{"-bootstrapsigner", "-tokencleaner"}},
					"v":           {Values: []string{"4"}},
				},
			},
			expected: k8s.ControllerManagerConfigSpec{
				Enabled: true,
				Image:   imagePostRemoval,
				Args: []string{
					"/usr/local/bin/kube-controller-manager",
					"--use-service-account-credentials",
					"--allocate-node-cidrs=true",
					"--authentication-kubeconfig=" + kubeconfigPath,
					"--authorization-kubeconfig=" + kubeconfigPath,
					"--bind-address=127.0.0.1",
					"--cluster-cidr=10.244.0.0/16,fd00::/64",
					"--cluster-signing-cert-file=" + caCrtPath,
					"--cluster-signing-key-file=" + caKeyPath,
					"--configure-cloud-routes=false",
					"--controllers=*,-bootstrapsigner,-tokencleaner",
					"--kubeconfig=" + kubeconfigPath,
					"--leader-elect=true",
					"--profiling=false",
					"--root-ca-file=" + caCrtPath,
					"--service-account-private-key-file=" + saKeyPath,
					"--service-cluster-ip-range=10.96.0.0/12,fd00:1::/108",
					"--terminated-pod-gc-threshold=100",
					"--tls-min-version=VersionTLS13",
					"--use-service-account-credentials=true",
					"--v=4",
				},
			},
		},
		{
			name: "extra args override defaults; pass-through fields preserved",
			input: k8s.ControllerManagerConfigSpec{
				Enabled:      true,
				Image:        imagePostRemoval,
				PodCIDRs:     []string{"10.244.0.0/16"},
				ServiceCIDRs: []string{"10.96.0.0/12"},
				ExtraArgs: map[string]k8s.ArgValues{
					"bind-address":                {Values: []string{"0.0.0.0"}},
					"terminated-pod-gc-threshold": {Values: []string{"50"}},
				},
				ExtraVolumes: []k8s.ExtraVolume{
					{Name: "foo", HostPath: "/var/foo", MountPath: "/foo", ReadOnly: true},
				},
				EnvironmentVariables: map[string]string{
					"GOMAXPROCS": "4",
				},
				Resources: k8s.Resources{
					Requests: map[string]string{"cpu": "200m", "memory": "512Mi"},
					Limits:   map[string]string{"cpu": "500m", "memory": "1Gi"},
				},
			},
			expected: k8s.ControllerManagerConfigSpec{
				Enabled: true,
				Image:   imagePostRemoval,
				ExtraVolumes: []k8s.ExtraVolume{
					{Name: "foo", HostPath: "/var/foo", MountPath: "/foo", ReadOnly: true},
				},
				EnvironmentVariables: map[string]string{
					"GOMAXPROCS": "4",
				},
				Resources: k8s.Resources{
					Requests: map[string]string{"cpu": "200m", "memory": "512Mi"},
					Limits:   map[string]string{"cpu": "500m", "memory": "1Gi"},
				},
				Args: []string{
					"/usr/local/bin/kube-controller-manager",
					"--use-service-account-credentials",
					"--allocate-node-cidrs=true",
					"--authentication-kubeconfig=" + kubeconfigPath,
					"--authorization-kubeconfig=" + kubeconfigPath,
					"--bind-address=0.0.0.0",
					"--cluster-cidr=10.244.0.0/16",
					"--cluster-signing-cert-file=" + caCrtPath,
					"--cluster-signing-key-file=" + caKeyPath,
					"--configure-cloud-routes=false",
					"--controllers=*",
					"--kubeconfig=" + kubeconfigPath,
					"--leader-elect=true",
					"--profiling=false",
					"--root-ca-file=" + caCrtPath,
					"--service-account-private-key-file=" + saKeyPath,
					"--service-cluster-ip-range=10.96.0.0/12",
					"--terminated-pod-gc-threshold=50",
					"--tls-min-version=VersionTLS13",
					"--use-service-account-credentials=true",
				},
			},
		},
	} {
		suite.Run(tt.name, func() {
			in := k8s.NewControllerManagerConfig(k8s.ControllerManagerConfigID)
			*in.TypedSpec() = tt.input

			if _, err := suite.State().Get(suite.Ctx(), in.Metadata()); err == nil {
				ctest.UpdateWithConflicts(suite, in, func(r *k8s.ControllerManagerConfig) error {
					*r.TypedSpec() = tt.input

					return nil
				})
			} else {
				suite.Create(in)
			}

			ctest.AssertResource(suite, k8s.FinalControllerManagerConfigID, func(r *k8s.ControllerManagerConfig, asrt *assert.Assertions) {
				asrt.Equal(tt.expected, *r.TypedSpec())
			})
		})
	}
}

func TestControlPlaneControllerManagerFinalSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ControlPlaneControllerManagerFinalSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneControllerManagerFinalController()))
			},
		},
	})
}

type ControlPlaneAPIServerFinalSuite struct {
	ctest.DefaultSuite
}

// nolint:gocyclo
func (suite *ControlPlaneAPIServerFinalSuite) TestTransform() {
	secretsDir := constants.KubernetesAPIServerSecretsDir
	configDir := constants.KubernetesAPIServerConfigDir
	auditLogPath := filepath.Join(constants.KubernetesAuditLogDir, "kube-apiserver.log")

	authnConfigPath := filepath.Join(configDir, "authentication-config.yaml")
	authzConfigPath := filepath.Join(configDir, "authorization-config.yaml")

	const image = "registry.k8s.io/kube-apiserver:v1.32.0"

	// commonArgs are the args shared by every case, excluding the authentication-config /
	// authorization-config / authorization-mode flags which differ per case. They are kept
	// in the sorted order produced by argsbuilder.Args().
	commonArgs := func() []string {
		return []string{
			"/usr/local/bin/kube-apiserver",
			"--admission-control-config-file=" + filepath.Join(configDir, "admission-control-config.yaml"),
			"--allow-privileged=true",
			"--api-audiences=https://localhost:6443",
			"--audit-log-maxage=30",
			"--audit-log-maxbackup=10",
			"--audit-log-maxsize=100",
			"--audit-log-path=" + auditLogPath,
			"--audit-policy-file=" + filepath.Join(configDir, "auditpolicy.yaml"),
			// authentication-config / authorization-* slotted in here per-case
			"--bind-address=0.0.0.0",
			"--client-ca-file=" + filepath.Join(secretsDir, "ca.crt"),
			"--enable-admission-plugins=NodeRestriction",
			"--enable-bootstrap-token-auth=true",
			"--encryption-provider-config=" + filepath.Join(secretsDir, "encryptionconfig.yaml"),
			"--etcd-cafile=" + filepath.Join(secretsDir, "etcd-client-ca.crt"),
			"--etcd-certfile=" + filepath.Join(secretsDir, "etcd-client.crt"),
			"--etcd-keyfile=" + filepath.Join(secretsDir, "etcd-client.key"),
			"--etcd-servers=https://127.0.0.1:2379",
			"--kubelet-client-certificate=" + filepath.Join(secretsDir, "apiserver-kubelet-client.crt"),
			"--kubelet-client-key=" + filepath.Join(secretsDir, "apiserver-kubelet-client.key"),
			"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
			"--profiling=false",
			"--proxy-client-cert-file=" + filepath.Join(secretsDir, "front-proxy-client.crt"),
			"--proxy-client-key-file=" + filepath.Join(secretsDir, "front-proxy-client.key"),
			"--requestheader-allowed-names=front-proxy-client",
			"--requestheader-client-ca-file=" + filepath.Join(secretsDir, "aggregator-ca.crt"),
			"--requestheader-extra-headers-prefix=X-Remote-Extra-",
			"--requestheader-group-headers=X-Remote-Group",
			"--requestheader-username-headers=X-Remote-User",
			"--secure-port=6443",
			"--service-account-issuer=https://localhost:6443",
			"--service-account-key-file=" + filepath.Join(secretsDir, "service-account.pub"),
			"--service-account-signing-key-file=" + filepath.Join(secretsDir, "service-account.key"),
			"--service-cluster-ip-range=10.96.0.0/12",
			"--tls-cert-file=" + filepath.Join(secretsDir, "apiserver.crt"),
			"--tls-min-version=VersionTLS13",
			"--tls-private-key-file=" + filepath.Join(secretsDir, "apiserver.key"),
		}
	}

	baseSpec := func() k8s.APIServerConfigSpec {
		return k8s.APIServerConfigSpec{
			Image:                image,
			ControlPlaneEndpoint: "https://localhost:6443",
			EtcdServers:          []string{"https://127.0.0.1:2379"},
			LocalPort:            6443,
			ServiceCIDRs:         []string{"10.96.0.0/12"},
		}
	}

	// withAuthArgs inserts the per-case authentication/authorization flags into the sorted
	// commonArgs at the right alphabetical positions (after --audit-policy-file).
	withAuthArgs := func(authArgs ...string) []string {
		args := commonArgs()
		out := make([]string, 0, len(args)+len(authArgs))

		for _, a := range args {
			if a == "--bind-address=0.0.0.0" {
				out = append(out, authArgs...)
			}

			out = append(out, a)
		}

		return out
	}

	// withAnonymousAuth inserts the legacy-path --anonymous-auth=false flag into its sorted
	// position (right after --allow-privileged=true). The legacy path (no authentication-config)
	// disables anonymous auth explicitly.
	withAnonymousAuth := func(args []string) []string {
		out := make([]string, 0, len(args)+1)

		for _, a := range args {
			out = append(out, a)

			if a == "--allow-privileged=true" {
				out = append(out, "--anonymous-auth=false")
			}
		}

		return out
	}

	for _, tt := range []struct {
		name     string
		input    k8s.APIServerConfigSpec
		expected k8s.APIServerConfigSpec
	}{
		{
			// New path: structured authentication & authorization config files are used.
			name: "new path: authentication-config and authorization-config",
			input: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = true

				return s
			}(),
			expected: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = true
				s.Args = withAuthArgs(
					"--authentication-config="+authnConfigPath,
					"--authorization-config="+authzConfigPath,
				)

				return s
			}(),
		},
		{
			// Legacy path: neither authentication-config nor authorization-config is present;
			// the user pins authorization-mode, so the controller falls back to --authorization-mode.
			name: "legacy path: no authentication-config, authorization-mode set",
			input: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = false
				s.ExtraArgs = map[string]k8s.ArgValues{
					"authorization-mode": {Values: []string{"Webhook"}},
				}

				return s
			}(),
			expected: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.Args = withAnonymousAuth(withAuthArgs(
					"--authorization-mode=Node,RBAC,Webhook",
				))

				return s
			}(),
		},
		{
			// Legacy path triggered via an authorization-webhook-* flag rather than authorization-mode.
			name: "legacy path: authorization-webhook flag triggers authorization-mode",
			input: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = false
				s.ExtraArgs = map[string]k8s.ArgValues{
					"authorization-webhook-config-file": {Values: []string{"/etc/kubernetes/webhook.yaml"}},
				}

				return s
			}(),
			expected: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				args := withAuthArgs(
					"--authorization-mode=Node,RBAC",
				)
				// authorization-webhook-config-file sorts right after --authorization-mode.
				out := make([]string, 0, len(args)+1)

				for _, a := range args {
					out = append(out, a)

					if a == "--authorization-mode=Node,RBAC" {
						out = append(out, "--authorization-webhook-config-file=/etc/kubernetes/webhook.yaml")
					}
				}

				s.Args = withAnonymousAuth(out)

				return s
			}(),
		},
		{
			// Pass-through fields, conditional advertise-address & cloud-provider, and additive extra args.
			name: "pass-through fields, advertised address, cloud provider and extra args",
			input: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = true
				s.CloudProvider = "external"
				s.AdvertisedAddress = "1.2.3.4"
				s.ExtraVolumes = []k8s.ExtraVolume{
					{Name: "foo", HostPath: "/var/foo", MountPath: "/foo", ReadOnly: true},
				}
				s.EnvironmentVariables = map[string]string{"GOMAXPROCS": "4"}
				s.Resources = k8s.Resources{
					Requests: map[string]string{"cpu": "200m", "memory": "1Gi"},
					Limits:   map[string]string{"cpu": "2", "memory": "4Gi"},
				}
				s.ExtraArgs = map[string]k8s.ArgValues{
					"enable-admission-plugins": {Values: []string{"PodNodeSelector"}},
					"v":                        {Values: []string{"4"}},
				}

				return s
			}(),
			expected: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = true
				s.CloudProvider = "external"
				s.AdvertisedAddress = "1.2.3.4"
				s.ExtraVolumes = []k8s.ExtraVolume{
					{Name: "foo", HostPath: "/var/foo", MountPath: "/foo", ReadOnly: true},
				}
				s.EnvironmentVariables = map[string]string{"GOMAXPROCS": "4"}
				s.Resources = k8s.Resources{
					Requests: map[string]string{"cpu": "200m", "memory": "1Gi"},
					Limits:   map[string]string{"cpu": "2", "memory": "4Gi"},
				}

				args := withAuthArgs(
					"--authentication-config="+authnConfigPath,
					"--authorization-config="+authzConfigPath,
				)
				// rebuild with advertise-address & cloud-provider slotted into their sorted positions,
				// the additive enable-admission-plugins value, and the trailing --v override.
				out := make([]string, 0, len(args)+3)

				for _, a := range args {
					switch a {
					case "--allow-privileged=true":
						out = append(out, "--advertise-address=1.2.3.4", a)
					case "--enable-admission-plugins=NodeRestriction":
						out = append(out, "--cloud-provider=external", "--enable-admission-plugins=NodeRestriction,PodNodeSelector")
					default:
						out = append(out, a)
					}
				}

				out = append(out, "--v=4")

				s.Args = out

				return s
			}(),
		},
		{
			// audit-log-max* flags use the default (override) merge policy, so user-provided
			// values replace the controller defaults.
			name: "audit-log-max flags overridden via extra args",
			input: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = true
				s.ExtraArgs = map[string]k8s.ArgValues{
					"audit-log-maxage":    {Values: []string{"7"}},
					"audit-log-maxbackup": {Values: []string{"3"}},
					"audit-log-maxsize":   {Values: []string{"50"}},
				}

				return s
			}(),
			expected: func() k8s.APIServerConfigSpec {
				s := baseSpec()
				s.UseAuthenticationConfig = true

				args := withAuthArgs(
					"--authentication-config="+authnConfigPath,
					"--authorization-config="+authzConfigPath,
				)

				overrides := map[string]string{
					"--audit-log-maxage=30":    "--audit-log-maxage=7",
					"--audit-log-maxbackup=10": "--audit-log-maxbackup=3",
					"--audit-log-maxsize=100":  "--audit-log-maxsize=50",
				}

				for i, a := range args {
					if replacement, ok := overrides[a]; ok {
						args[i] = replacement
					}
				}

				s.Args = args

				return s
			}(),
		},
	} {
		suite.Run(tt.name, func() {
			in := k8s.NewAPIServerConfig(k8s.APIServerConfigID)
			*in.TypedSpec() = tt.input

			if _, err := suite.State().Get(suite.Ctx(), in.Metadata()); err == nil {
				ctest.UpdateWithConflicts(suite, in, func(r *k8s.APIServerConfig) error {
					*r.TypedSpec() = tt.input

					return nil
				})
			} else {
				suite.Create(in)
			}

			ctest.AssertResource(suite, k8s.FinalAPIServerConfigID, func(r *k8s.APIServerConfig, asrt *assert.Assertions) {
				asrt.Equal(tt.expected, *r.TypedSpec())
			})
		})
	}
}

// TestRejectsDeniedExtraArgs verifies that extra args which collide with a MergeDenied
// policy cause the transform to fail, so no final APIServerConfig is produced.
func (suite *ControlPlaneAPIServerFinalSuite) TestRejectsDeniedExtraArgs() {
	for _, arg := range []string{
		"tls-min-version",
		"tls-cert-file",
		"proxy-client-key-file",
		"etcd-servers",
	} {
		suite.Run(arg, func() {
			in := k8s.NewAPIServerConfig(k8s.APIServerConfigID)
			*in.TypedSpec() = k8s.APIServerConfigSpec{
				Image:                "registry.k8s.io/kube-apiserver:v1.32.0",
				ControlPlaneEndpoint: "https://localhost:6443",
				EtcdServers:          []string{"https://127.0.0.1:2379"},
				LocalPort:            6443,
				ServiceCIDRs:         []string{"10.96.0.0/12"},
				ExtraArgs: map[string]k8s.ArgValues{
					arg: {Values: []string{"override"}},
				},
			}

			if _, err := suite.State().Get(suite.Ctx(), in.Metadata()); err == nil {
				ctest.UpdateWithConflicts(suite, in, func(r *k8s.APIServerConfig) error {
					*r.TypedSpec() = *in.TypedSpec()

					return nil
				})
			} else {
				suite.Create(in)
			}

			ctest.AssertNoResource[*k8s.APIServerConfig](suite, k8s.FinalAPIServerConfigID)
		})
	}
}

func TestControlPlaneAPIServerFinalSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ControlPlaneAPIServerFinalSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneAPIServerFinalController()))
			},
		},
	})
}
