// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package config_test

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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	configctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/config"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

type K8sControlPlaneSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *K8sControlPlaneSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&configctrl.K8sControlPlaneController{}))

	suite.startRuntime()
}

func (suite *K8sControlPlaneSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *K8sControlPlaneSuite) assertControlPlaneConfigs(resourceTypes ...resource.Type) error {
	for _, resourceType := range resourceTypes {
		resources, err := suite.state.List(
			suite.ctx,
			resource.NewMetadata(k8s.ControlPlaneNamespaceName, resourceType, "", resource.VersionUndefined),
		)
		if err != nil {
			return err
		}

		if len(resources.Items) == 0 {
			return retry.ExpectedError(fmt.Errorf("no resources with type %q found", resourceType))
		}
	}

	return nil
}

// setupMachine creates a machine with given configuration, waits for it to become ready,
// and returns API server's spec.
func (suite *K8sControlPlaneSuite) setupMachine(cfg *config.MachineConfig) k8s.APIServerConfigSpec {
	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)

	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))
	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertControlPlaneConfigs(
					k8s.AdmissionControlConfigType,
					k8s.AuditPolicyConfigType,
					k8s.APIServerConfigType,
					k8s.ControllerManagerConfigType,
					k8s.SchedulerConfigType,
					k8s.BootstrapManifestsConfigType,
					k8s.ExtraManifestsConfigType,
				)
			},
		),
	)

	r, err := suite.state.Get(suite.ctx, k8s.NewAPIServerConfig().Metadata())
	suite.Require().NoError(err)

	cp, ok := r.(*k8s.APIServerConfig)
	suite.Require().True(ok, "got %T", r)

	return *cp.TypedSpec()
}

func (suite *K8sControlPlaneSuite) TestReconcileDefaults() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{
					Endpoint: &v1alpha1.Endpoint{
						URL: u,
					},
				},
			},
		},
	)

	apiServerCfg := suite.setupMachine(cfg)
	suite.Assert().Empty(apiServerCfg.CloudProvider)

	r, err := suite.state.Get(suite.ctx, k8s.NewControllerManagerConfig().Metadata())
	suite.Require().NoError(err)
	suite.Assert().Empty(r.(*k8s.ControllerManagerConfig).TypedSpec().CloudProvider)

	bootstrapConfig, err := safe.StateGetResource(suite.ctx, suite.state, k8s.NewBootstrapManifestsConfig())
	suite.Require().NoError(err)

	suite.Assert().Equal("10.96.0.10", bootstrapConfig.TypedSpec().DNSServiceIP)
	suite.Assert().Equal("", bootstrapConfig.TypedSpec().DNSServiceIPv6)
}

func (suite *K8sControlPlaneSuite) TestReconcileIPv6() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
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
	)

	suite.setupMachine(cfg)

	bootstrapConfig, err := safe.StateGetResource(suite.ctx, suite.state, k8s.NewBootstrapManifestsConfig())
	suite.Require().NoError(err)

	suite.Assert().Equal("", bootstrapConfig.TypedSpec().DNSServiceIP)
	suite.Assert().Equal("fc00:db8:20::a", bootstrapConfig.TypedSpec().DNSServiceIPv6)
}

func (suite *K8sControlPlaneSuite) TestReconcileDualStack() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
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
	)

	suite.setupMachine(cfg)

	bootstrapConfig, err := safe.StateGetResource(suite.ctx, suite.state, k8s.NewBootstrapManifestsConfig())
	suite.Require().NoError(err)

	suite.Assert().Equal("10.96.0.10", bootstrapConfig.TypedSpec().DNSServiceIP)
	suite.Assert().Equal("fc00:db8:20::a", bootstrapConfig.TypedSpec().DNSServiceIPv6)
}

func (suite *K8sControlPlaneSuite) TestReconcileExtraVolumes() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
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
	)

	apiServerCfg := suite.setupMachine(cfg)
	suite.Assert().Equal(
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
}

func (suite *K8sControlPlaneSuite) TestReconcileEnvironment() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
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
	)

	apiServerCfg := suite.setupMachine(cfg)
	suite.Assert().Equal(
		map[string]string{
			"HTTP_PROXY": "foo",
		}, apiServerCfg.EnvironmentVariables,
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileExternalCloudProvider() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
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
	)

	apiServerCfg := suite.setupMachine(cfg)
	suite.Assert().Equal("external", apiServerCfg.CloudProvider)

	r, err := suite.state.Get(suite.ctx, k8s.NewControllerManagerConfig().Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal("external", r.(*k8s.ControllerManagerConfig).TypedSpec().CloudProvider)

	r, err = suite.state.Get(suite.ctx, k8s.NewExtraManifestsConfig().Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(
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
		}, r.(*k8s.ExtraManifestsConfig).TypedSpec(),
	)
}

func (suite *K8sControlPlaneSuite) TestReconcileInlineManifests() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
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
	)

	suite.setupMachine(cfg)

	r, err := suite.state.Get(suite.ctx, k8s.NewExtraManifestsConfig().Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(
		&k8s.ExtraManifestsConfigSpec{
			ExtraManifests: []k8s.ExtraManifest{
				{
					Name:           "namespace-ci",
					Priority:       "99",
					InlineManifest: "apiVersion: v1\nkind: Namespace\nmetadata:\n\tname: ci",
				},
			},
		}, r.(*k8s.ExtraManifestsConfig).TypedSpec(),
	)
}

func (suite *K8sControlPlaneSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, "-"),
		),
	)
	suite.Assert().NoError(
		suite.state.Destroy(
			context.Background(),
			k8s.NewAPIServerConfig().Metadata(),
			state.WithDestroyOwner("config.K8sControlPlaneController"),
		),
	)
}

func TestK8sControlPlaneSuite(t *testing.T) {
	suite.Run(t, new(K8sControlPlaneSuite))
}
