// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type K8sControlPlaneSuite struct {
	ctest.DefaultSuite
}

// setupMachine creates a machine with given configuration, waits for it to become ready,
// and returns API server's spec.
func (suite *K8sControlPlaneSuite) setupMachine(cfg *config.MachineConfig) {
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.AdmissionControlConfigID}, func(*k8s.AdmissionControlConfig, *assert.Assertions) {})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.AuditPolicyConfigID}, func(*k8s.AuditPolicyConfig, *assert.Assertions) {})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.AuthorizationConfigID}, func(*k8s.AuthorizationConfig, *assert.Assertions) {})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.APIServerConfigID}, func(*k8s.APIServerConfig, *assert.Assertions) {})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ControllerManagerConfigID}, func(*k8s.ControllerManagerConfig, *assert.Assertions) {})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.SchedulerConfigID}, func(*k8s.SchedulerConfig, *assert.Assertions) {})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.BootstrapManifestsConfigID}, func(*k8s.BootstrapManifestsConfig, *assert.Assertions) {})
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ExtraManifestsConfigID}, func(*k8s.ExtraManifestsConfig, *assert.Assertions) {})
}

func (suite *K8sControlPlaneSuite) TestReconcileDefaults() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.APIServerConfigID},
		func(apiServer *k8s.APIServerConfig, assert *assert.Assertions) {
			apiServerCfg := apiServer.TypedSpec()

			assert.Empty(apiServerCfg.CloudProvider)
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ControllerManagerConfigID},
		func(controllerManager *k8s.ControllerManagerConfig, assert *assert.Assertions) {
			assert.Empty(controllerManager.TypedSpec().CloudProvider)
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.BootstrapManifestsConfigID},
		func(bootstrapConfig *k8s.BootstrapManifestsConfig, assert *assert.Assertions) {
			assert.Equal("10.96.0.10", bootstrapConfig.TypedSpec().DNSServiceIP)
			assert.Equal("", bootstrapConfig.TypedSpec().DNSServiceIPv6)
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.AuthorizationConfigID},
		func(authorizationConfig *k8s.AuthorizationConfig, assert *assert.Assertions) {
			assert.Equal(v1alpha1.APIServerDefaultAuthorizationConfigAuthorizers, authorizationConfig.TypedSpec().Config)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileEmptyAuthorizationConfigForK8sLessThanv128() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						ContainerImage:            "k8s.gcr.io/kube-apiserver:v1.28.0",
						AuthorizationConfigConfig: []*v1alpha1.AuthorizationConfigAuthorizerConfig{},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResource[*k8s.AuthorizationConfig](suite.Ctx(), suite.T(), suite.State(), k8s.AuthorizationConfigID, func(authorizationConfig *k8s.AuthorizationConfig, assert *assert.Assertions) {
		assert.Equal(&k8s.AuthorizationConfigSpec{
			Image: "k8s.gcr.io/kube-apiserver:v1.28.0",
		}, authorizationConfig.TypedSpec())
	})
}

func (suite *K8sControlPlaneSuite) TestReconcileEmptyAuthorizationConfigAuthorizers() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						AuthorizationConfigConfig: []*v1alpha1.AuthorizationConfigAuthorizerConfig{},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.AuthorizationConfigID},
		func(authorizationConfig *k8s.AuthorizationConfig, assert *assert.Assertions) {
			assert.Equal(v1alpha1.APIServerDefaultAuthorizationConfigAuthorizers, authorizationConfig.TypedSpec().Config)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileAdditionalAuthorizationConfigAuthorizers() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						AuthorizationConfigConfig: []*v1alpha1.AuthorizationConfigAuthorizerConfig{
							{
								AuthorizerType: "Webhook",
								AuthorizerName: "webhook",
								AuthorizerWebhook: v1alpha1.Unstructured{
									Object: map[string]any{
										"timeout":                    "3s",
										"subjectAccessReviewVersion": "v1",
										"matchConditionSubjectAccessReviewVersion": "v1",
										"failurePolicy": "NoOpinion",
										"connectionInfo": map[string]any{
											"type": "InClusterConfig",
										},
									},
								},
							},
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	expectedAuthorizers := append(v1alpha1.APIServerDefaultAuthorizationConfigAuthorizers, k8s.AuthorizationAuthorizersSpec{ //nolint:gocritic
		Type: "Webhook",
		Name: "webhook",
		Webhook: map[string]any{
			"timeout":                    "3s",
			"subjectAccessReviewVersion": "v1",
			"matchConditionSubjectAccessReviewVersion": "v1",
			"failurePolicy": "NoOpinion",
			"connectionInfo": map[string]any{
				"type": "InClusterConfig",
			},
		},
	})

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.AuthorizationConfigID},
		func(authorizationConfig *k8s.AuthorizationConfig, assert *assert.Assertions) {
			assert.Equal(expectedAuthorizers, authorizationConfig.TypedSpec().Config)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileAdditionalAuthorizationConfigAuthorizersWithDefaultsSet() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						AuthorizationConfigConfig: []*v1alpha1.AuthorizationConfigAuthorizerConfig{
							{
								AuthorizerType: "RBAC",
								AuthorizerName: "foo",
							},
							{
								AuthorizerType: "Webhook",
								AuthorizerName: "webhook",
								AuthorizerWebhook: v1alpha1.Unstructured{
									Object: map[string]any{
										"timeout":                    "3s",
										"subjectAccessReviewVersion": "v1",
										"matchConditionSubjectAccessReviewVersion": "v1",
										"failurePolicy": "NoOpinion",
										"connectionInfo": map[string]any{
											"type": "InClusterConfig",
										},
									},
								},
							},
							{
								AuthorizerType: "Node",
								AuthorizerName: "foo",
							},
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	expectedAuthorizers := append(v1alpha1.APIServerDefaultAuthorizationConfigAuthorizers, k8s.AuthorizationAuthorizersSpec{ //nolint:gocritic
		Type: "Webhook",
		Name: "webhook",
		Webhook: map[string]any{
			"timeout":                    "3s",
			"subjectAccessReviewVersion": "v1",
			"matchConditionSubjectAccessReviewVersion": "v1",
			"failurePolicy": "NoOpinion",
			"connectionInfo": map[string]any{
				"type": "InClusterConfig",
			},
		},
	})

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.AuthorizationConfigID},
		func(authorizationConfig *k8s.AuthorizationConfig, assert *assert.Assertions) {
			assert.Equal(expectedAuthorizers, authorizationConfig.TypedSpec().Config)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileTransitionWorker() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	cfg.Container().RawV1Alpha1().MachineConfig.MachineType = "worker"
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	rtestutils.AssertNoResource[*k8s.AdmissionControlConfig](suite.Ctx(), suite.T(), suite.State(), k8s.AdmissionControlConfigID)
	rtestutils.AssertNoResource[*k8s.AuditPolicyConfig](suite.Ctx(), suite.T(), suite.State(), k8s.AuditPolicyConfigID)
	rtestutils.AssertNoResource[*k8s.AuthorizationConfig](suite.Ctx(), suite.T(), suite.State(), k8s.AuthorizationConfigID)
	rtestutils.AssertNoResource[*k8s.APIServerConfig](suite.Ctx(), suite.T(), suite.State(), k8s.APIServerConfigID)
	rtestutils.AssertNoResource[*k8s.ControllerManagerConfig](suite.Ctx(), suite.T(), suite.State(), k8s.ControllerManagerConfigID)
	rtestutils.AssertNoResource[*k8s.SchedulerConfig](suite.Ctx(), suite.T(), suite.State(), k8s.SchedulerConfigID)
	rtestutils.AssertNoResource[*k8s.BootstrapManifestsConfig](suite.Ctx(), suite.T(), suite.State(), k8s.BootstrapManifestsConfigID)
	rtestutils.AssertNoResource[*k8s.ExtraManifestsConfig](suite.Ctx(), suite.T(), suite.State(), k8s.ExtraManifestsConfigID)
}

func (suite *K8sControlPlaneSuite) TestReconcileIPv6() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						PodSubnet:     []string{constants.DefaultIPv6PodNet},
						ServiceSubnet: []string{constants.DefaultIPv6ServiceNet},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.BootstrapManifestsConfigID},
		func(bootstrapConfig *k8s.BootstrapManifestsConfig, assert *assert.Assertions) {
			assert.Equal("", bootstrapConfig.TypedSpec().DNSServiceIP)
			assert.Equal("fc00:db8:20::a", bootstrapConfig.TypedSpec().DNSServiceIPv6)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileDualStack() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						PodSubnet:     []string{constants.DefaultIPv4PodNet, constants.DefaultIPv6PodNet},
						ServiceSubnet: []string{constants.DefaultIPv4ServiceNet, constants.DefaultIPv6ServiceNet},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.BootstrapManifestsConfigID},
		func(bootstrapConfig *k8s.BootstrapManifestsConfig, assert *assert.Assertions) {
			assert.Equal("10.96.0.10", bootstrapConfig.TypedSpec().DNSServiceIP)
			assert.Equal("fc00:db8:20::a", bootstrapConfig.TypedSpec().DNSServiceIPv6)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileExtraVolumes() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						ExtraVolumesConfig: []v1alpha1.VolumeMountConfig{
							{
								VolumeHostPath:  "/var/lib",
								VolumeMountPath: "/var/foo/",
							},
							{
								VolumeHostPath:  "/var/lib/a.foo",
								VolumeMountPath: "/var/foo/b.foo",
							},
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.APIServerConfigID},
		func(apiServer *k8s.APIServerConfig, assert *assert.Assertions) {
			apiServerCfg := apiServer.TypedSpec()

			assert.Equal(
				[]k8s.ExtraVolume{
					{
						Name:      "var-foo",
						HostPath:  "/var/lib",
						MountPath: "/var/foo/",
						ReadOnly:  false,
					},
					{
						Name:      "var-foo-b-foo",
						HostPath:  "/var/lib/a.foo",
						MountPath: "/var/foo/b.foo",
						ReadOnly:  false,
					},
				}, apiServerCfg.ExtraVolumes,
			)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileEnvironment() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						EnvConfig: v1alpha1.Env{
							"HTTP_PROXY": "foo",
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.APIServerConfigID},
		func(apiServer *k8s.APIServerConfig, assert *assert.Assertions) {
			apiServerCfg := apiServer.TypedSpec()

			assert.Equal(
				map[string]string{
					"HTTP_PROXY": "foo",
				}, apiServerCfg.EnvironmentVariables,
			)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileResources() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						ResourcesConfig: &v1alpha1.ResourcesConfig{
							Requests: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu":    "100m",
									"memory": "1Gi",
								},
							},
							Limits: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu":    2,
									"memory": "1500Mi",
								},
							},
						},
					},
					ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{
						ResourcesConfig: &v1alpha1.ResourcesConfig{
							Requests: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu":    "50m",
									"memory": "500Mi",
								},
							},
							Limits: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu":    1,
									"memory": "1000Mi",
								},
							},
						},
					},
					SchedulerConfig: &v1alpha1.SchedulerConfig{
						ResourcesConfig: &v1alpha1.ResourcesConfig{
							Requests: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu":    "150m",
									"memory": "2Gi",
								},
							},
							Limits: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu":    3,
									"memory": "2000Mi",
								},
							},
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.APIServerConfigID},
		func(apiServer *k8s.APIServerConfig, assert *assert.Assertions) {
			apiServerCfg := apiServer.TypedSpec()

			assert.Equal(
				k8s.Resources{
					Requests: map[string]string{
						"cpu":    "100m",
						"memory": "1Gi",
					},
					Limits: map[string]string{
						"cpu":    "2",
						"memory": "1500Mi",
					},
				}, apiServerCfg.Resources,
			)
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ControllerManagerConfigID},
		func(controllerManager *k8s.ControllerManagerConfig, assert *assert.Assertions) {
			controllerManagerCfg := controllerManager.TypedSpec()

			assert.Equal(
				k8s.Resources{
					Requests: map[string]string{
						"cpu":    "50m",
						"memory": "500Mi",
					},
					Limits: map[string]string{
						"cpu":    "1",
						"memory": "1000Mi",
					},
				}, controllerManagerCfg.Resources,
			)
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.SchedulerConfigID},
		func(scheduler *k8s.SchedulerConfig, assert *assert.Assertions) {
			schedulerCfg := scheduler.TypedSpec()

			assert.Equal(
				k8s.Resources{
					Requests: map[string]string{
						"cpu":    "150m",
						"memory": "2Gi",
					},
					Limits: map[string]string{
						"cpu":    "3",
						"memory": "2000Mi",
					},
				}, schedulerCfg.Resources,
			)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileExternalCloudProvider() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: pointer.To(true),
						ExternalManifests: []string{
							"https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml",
							"https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml",
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.APIServerConfigID},
		func(apiServer *k8s.APIServerConfig, assert *assert.Assertions) {
			apiServerCfg := apiServer.TypedSpec()

			assert.Equal("external", apiServerCfg.CloudProvider)
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ControllerManagerConfigID},
		func(controllerManager *k8s.ControllerManagerConfig, assert *assert.Assertions) {
			assert.Equal("external", controllerManager.TypedSpec().CloudProvider)
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ExtraManifestsConfigID},
		func(extraManifests *k8s.ExtraManifestsConfig, assert *assert.Assertions) {
			assert.Equal(
				&k8s.ExtraManifestsConfigSpec{
					ExtraManifests: []k8s.ExtraManifest{
						{
							Name:     "https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml",
							URL:      "https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml",
							Priority: "30",
						},
						{
							Name:     "https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml",
							URL:      "https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml",
							Priority: "30",
						},
					},
				}, extraManifests.TypedSpec())
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileInlineManifests() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ClusterInlineManifests: v1alpha1.ClusterInlineManifests{
						{
							InlineManifestName: "namespace-ci",
							InlineManifestContents: strings.TrimSpace(
								`
apiVersion: v1
kind: Namespace
metadata:
	name: ci
`,
							),
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.ExtraManifestsConfigID},
		func(extraManifests *k8s.ExtraManifestsConfig, assert *assert.Assertions) {
			assert.Equal(
				&k8s.ExtraManifestsConfigSpec{
					ExtraManifests: []k8s.ExtraManifest{
						{
							Name:           "namespace-ci",
							Priority:       "99",
							InlineManifest: "apiVersion: v1\nkind: Namespace\nmetadata:\n\tname: ci",
						},
					},
				},
				extraManifests.TypedSpec())
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileKubeProxyMode() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.BootstrapManifestsConfigID},
		func(cfg *k8s.BootstrapManifestsConfig, assert *assert.Assertions) {
			assert.Contains(
				cfg.TypedSpec().ProxyArgs,
				"--proxy-mode=nftables",
			)
		},
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileKubeProxyModeLegacy() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ProxyConfig: &v1alpha1.ProxyConfig{
						ContainerImage: constants.KubeProxyImage + ":v1.30.0",
					},
				},
			},
		),
	)

	suite.setupMachine(cfg)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.BootstrapManifestsConfigID},
		func(cfg *k8s.BootstrapManifestsConfig, assert *assert.Assertions) {
			assert.Contains(
				cfg.TypedSpec().ProxyArgs,
				"--proxy-mode=iptables",
			)
		},
	)
}

func TestK8sControlPlaneSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &K8sControlPlaneSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneAPIServerController()))
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneAdmissionControlController()))
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneAuditPolicyController()))
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneAuthorizationController()))
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneBootstrapManifestsController()))
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneControllerManagerController()))
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneExtraManifestsController()))
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewControlPlaneSchedulerController()))
			},
		},
	})
}
