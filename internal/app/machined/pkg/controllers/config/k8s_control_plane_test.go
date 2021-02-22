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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"github.com/talos-systems/os-runtime/pkg/controller/runtime"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/inmem"
	"github.com/talos-systems/os-runtime/pkg/state/impl/namespaced"

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

func (suite *K8sControlPlaneSuite) TestReconcileDefaults() {
	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewV1Alpha1(&v1alpha1.Config{
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
}

func (suite *K8sControlPlaneSuite) TestReconcileExtraVolumes() {
	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewV1Alpha1(&v1alpha1.Config{
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

	apiServerCfg := r.(*config.K8sControlPlane).APIServer()

	suite.Assert().Equal([]config.K8sExtraVolume{
		{
			Name:      "var-foo",
			HostPath:  "/var/lib",
			MountPath: "/var/foo/",
			ReadOnly:  false,
		},
	}, apiServerCfg.ExtraVolumes)
}

func (suite *K8sControlPlaneSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, "-")))
	suite.Assert().NoError(suite.state.Destroy(context.Background(), config.NewK8sControlPlaneAPIServer().Metadata()))
}

func TestK8sControlPlaneSuite(t *testing.T) {
	suite.Run(t, new(K8sControlPlaneSuite))
}
