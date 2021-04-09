// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strings"
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

	configctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/k8s"
)

type K8sControlPlaneSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *K8sControlPlaneSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	logger := log.New(log.Writer(), "controller-runtime: ", log.Flags())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
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

func (suite *K8sControlPlaneSuite) assertK8sControlPlanes(manifests []string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(config.NamespaceName, config.K8sControlPlaneType, "", resource.VersionUndefined))
	if err != nil {
		return retry.UnexpectedError(err)
	}

	ids := make([]string, 0, len(resources.Items))

	for _, res := range resources.Items {
		ids = append(ids, res.Metadata().ID())
	}

	if !reflect.DeepEqual(manifests, ids) {
		return retry.ExpectedError(fmt.Errorf("expected %q, got %q", manifests, ids))
	}

	return nil
}

// setupMachine creates a machine with given configuration, waits for it to become ready,
// and returns API server's spec.
func (suite *K8sControlPlaneSuite) setupMachine(cfg *config.MachineConfig) config.K8sControlPlaneAPIServerSpec {
	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)

	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))
	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertK8sControlPlanes(
				[]string{
					config.K8sExtraManifestsID,
					config.K8sControlPlaneAPIServerID,
					config.K8sControlPlaneControllerManagerID,
					config.K8sControlPlaneSchedulerID,
					config.K8sManifestsID,
				},
			)
		},
	))

	r, err := suite.state.Get(suite.ctx, config.NewK8sControlPlaneAPIServer().Metadata())
	suite.Require().NoError(err)

	cp, ok := r.(*config.K8sControlPlane)
	suite.Require().True(ok, "got %T", r)

	return cp.APIServer()
}

func (suite *K8sControlPlaneSuite) TestReconcileDefaults() {
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

	apiServerCfg := suite.setupMachine(cfg)
	suite.Assert().Empty(apiServerCfg.CloudProvider)

	r, err := suite.state.Get(suite.ctx, config.NewK8sControlPlaneControllerManager().Metadata())
	suite.Require().NoError(err)
	suite.Assert().Empty(r.(*config.K8sControlPlane).ControllerManager().CloudProvider)
}

func (suite *K8sControlPlaneSuite) TestReconcileExtraVolumes() {
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
			APIServerConfig: &v1alpha1.APIServerConfig{
				ExtraVolumesConfig: []v1alpha1.VolumeMountConfig{
					{
						VolumeHostPath:  "/var/lib",
						VolumeMountPath: "/var/foo/",
					},
				},
			},
		},
	})

	apiServerCfg := suite.setupMachine(cfg)
	suite.Assert().Equal([]config.K8sExtraVolume{
		{
			Name:      "var-foo",
			HostPath:  "/var/lib",
			MountPath: "/var/foo/",
			ReadOnly:  false,
		},
	}, apiServerCfg.ExtraVolumes)
}

func (suite *K8sControlPlaneSuite) TestReconcileExternalCloudProvider() {
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
			ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
				ExternalEnabled: true,
				ExternalManifests: []string{
					"https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml",
					"https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml",
				},
			},
		},
	})

	apiServerCfg := suite.setupMachine(cfg)
	suite.Assert().Equal("external", apiServerCfg.CloudProvider)

	r, err := suite.state.Get(suite.ctx, config.NewK8sControlPlaneControllerManager().Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal("external", r.(*config.K8sControlPlane).ControllerManager().CloudProvider)

	r, err = suite.state.Get(suite.ctx, config.NewK8sExtraManifests().Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(config.K8sExtraManifestsSpec{
		ExtraManifests: []config.ExtraManifest{
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
	}, r.(*config.K8sControlPlane).ExtraManifests())
}

func (suite *K8sControlPlaneSuite) TestReconcileInlineManifests() {
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
			ClusterInlineManifests: v1alpha1.ClusterInlineManifests{
				{
					InlineManifestName: "namespace-ci",
					InlineManifestContents: strings.TrimSpace(`
apiVersion: v1
kind: Namespace
metadata:
	name: ci
`),
				},
			},
		},
	})

	suite.setupMachine(cfg)

	r, err := suite.state.Get(suite.ctx, config.NewK8sExtraManifests().Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(config.K8sExtraManifestsSpec{
		ExtraManifests: []config.ExtraManifest{
			{
				Name:           "namespace-ci",
				Priority:       "99",
				InlineManifest: "apiVersion: v1\nkind: Namespace\nmetadata:\n\tname: ci",
			},
		},
	}, r.(*config.K8sControlPlane).ExtraManifests())
}

func (suite *K8sControlPlaneSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, "-")))
	suite.Assert().NoError(suite.state.Destroy(context.Background(), config.NewK8sControlPlaneAPIServer().Metadata(), state.WithDestroyOwner("config.K8sControlPlaneController")))
}

func TestK8sControlPlaneSuite(t *testing.T) {
	suite.Run(t, new(K8sControlPlaneSuite))
}
