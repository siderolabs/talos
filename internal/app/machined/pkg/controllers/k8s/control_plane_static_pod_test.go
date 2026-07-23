// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8stemplates"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type ControlPlaneStaticPodSuite struct {
	ctest.DefaultSuite
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileDefaults() {
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig(k8s.FinalAPIServerConfigID)
	configControllerManager := k8s.NewControllerManagerConfig(k8s.FinalControllerManagerConfigID)
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig(k8s.FinalSchedulerConfigID)
	configScheduler.TypedSpec().Enabled = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), secretStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configAPIServer))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configControllerManager))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configScheduler))

	rtestutils.AssertResources(
		suite.Ctx(),
		suite.T(),
		suite.State(),
		[]resource.ID{
			k8s.APIServerID,
			k8s.ControllerManagerID,
			k8s.SchedulerID,
		},
		func(staticPod *k8s.StaticPod, asrt *assert.Assertions) {
			_, err := k8sadapter.StaticPod(staticPod).Pod()
			suite.Require().NoError(err)
		},
	)
}

func (suite *ControlPlaneStaticPodSuite) TestEtcdUnhealthyPreservesStaticPods() {
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig(k8s.FinalAPIServerConfigID)
	configControllerManager := k8s.NewControllerManagerConfig(k8s.FinalControllerManagerConfigID)
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig(k8s.FinalSchedulerConfigID)
	configScheduler.TypedSpec().Enabled = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), secretStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configAPIServer))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configControllerManager))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configScheduler))

	staticPodIDs := []resource.ID{
		k8s.APIServerID,
		k8s.ControllerManagerID,
		k8s.SchedulerID,
	}

	rtestutils.AssertResources(
		suite.Ctx(),
		suite.T(),
		suite.State(),
		staticPodIDs,
		func(*k8s.StaticPod, *assert.Assertions) {},
	)

	ctest.UpdateWithConflicts(suite, v1alpha1.NewService("etcd"), func(service *v1alpha1.Service) error {
		service.TypedSpec().Healthy = false
		service.TypedSpec().Unknown = false

		return nil
	})

	ctx := suite.Ctx()
	st := suite.State()

	for _, id := range staticPodIDs {
		suite.Never(func() bool {
			_, err := st.Get(ctx, k8s.NewStaticPod(k8s.NamespaceName, id).Metadata())

			return state.IsNotFoundError(err)
		}, 500*time.Millisecond, 10*time.Millisecond)
	}
}

func (suite *ControlPlaneStaticPodSuite) TestControlPlaneStaticPodsExceptScheduler() {
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig(k8s.FinalAPIServerConfigID)
	configControllerManager := k8s.NewControllerManagerConfig(k8s.FinalControllerManagerConfigID)
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig(k8s.FinalSchedulerConfigID)
	configScheduler.TypedSpec().Enabled = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), secretStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configAPIServer))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configControllerManager))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configScheduler))

	rtestutils.AssertResources(
		suite.Ctx(),
		suite.T(),
		suite.State(),
		[]resource.ID{
			k8s.APIServerID,
			k8s.ControllerManagerID,
			k8s.SchedulerID,
		},
		func(*k8s.StaticPod, *assert.Assertions) {},
	)

	configScheduler.TypedSpec().Enabled = false
	suite.Require().NoError(suite.State().Update(suite.Ctx(), configScheduler))

	rtestutils.AssertResources(
		suite.Ctx(),
		suite.T(),
		suite.State(),
		[]resource.ID{
			k8s.APIServerID,
			k8s.ControllerManagerID,
		},
		func(*k8s.StaticPod, *assert.Assertions) {},
	)
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileStaticPodResources() {
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)

	configAPIServer := k8s.NewAPIServerConfig(k8s.FinalAPIServerConfigID)
	configControllerManager := k8s.NewControllerManagerConfig(k8s.FinalControllerManagerConfigID)
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig(k8s.FinalSchedulerConfigID)
	configScheduler.TypedSpec().Enabled = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), secretStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configAPIServer))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configControllerManager))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configScheduler))

	tests := []struct {
		resources   k8s.Resources
		expected    v1.ResourceRequirements
		expectedEnv v1.EnvVar
	}{
		{
			resources: k8s.Resources{
				Requests: map[string]string{
					string(v1.ResourceCPU):    "100m",
					string(v1.ResourceMemory): "256Mi",
				},
			},
			expected: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]apiresource.Quantity{
					v1.ResourceCPU:    apiresource.MustParse("100m"),
					v1.ResourceMemory: apiresource.MustParse("256Mi"),
				},
			},
		},
		{
			resources: k8s.Resources{
				Requests: map[string]string{
					string(v1.ResourceCPU):    "100m",
					string(v1.ResourceMemory): "256Mi",
				},
				Limits: map[string]string{
					string(v1.ResourceCPU):    "1",
					string(v1.ResourceMemory): "1Gi",
				},
			},
			expected: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]apiresource.Quantity{
					v1.ResourceCPU:    apiresource.MustParse("100m"),
					v1.ResourceMemory: apiresource.MustParse("256Mi"),
				},
				Limits: map[v1.ResourceName]apiresource.Quantity{
					v1.ResourceCPU:    apiresource.MustParse("1"),
					v1.ResourceMemory: apiresource.MustParse("1Gi"),
				},
			},
			expectedEnv: v1.EnvVar{
				Name:  "GOMEMLIMIT",
				Value: strconv.FormatInt(1024*1024*1024*k8stemplates.GoGCMemLimitPercentage/100, 10),
			},
		},
	}
	for _, test := range tests {
		configAPIServer.TypedSpec().Resources = test.resources
		configControllerManager.TypedSpec().Resources = test.resources
		configScheduler.TypedSpec().Resources = test.resources

		suite.Require().NoError(suite.State().Update(suite.Ctx(), configAPIServer))
		suite.Require().NoError(suite.State().Update(suite.Ctx(), configControllerManager))
		suite.Require().NoError(suite.State().Update(suite.Ctx(), configScheduler))

		rtestutils.AssertResources(
			suite.Ctx(),
			suite.T(),
			suite.State(),
			[]resource.ID{
				k8s.APIServerID,
				k8s.ControllerManagerID,
				k8s.SchedulerID,
			},
			func(staticPod *k8s.StaticPod, assert *assert.Assertions) {
				pod, err := k8sadapter.StaticPod(staticPod).Pod()
				suite.Require().NoError(err)

				assert.NotEmpty(pod.Spec.Containers)

				assert.Equal(test.expected, pod.Spec.Containers[0].Resources)

				if test.expectedEnv.Name != "" {
					assert.Contains(pod.Spec.Containers[0].Env, test.expectedEnv)
				}
			},
		)
	}
}

func TestControlPlaneStaticPodSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ControlPlaneStaticPodSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.ControlPlaneStaticPodController{}))

				etcdService := v1alpha1.NewService("etcd")
				etcdService.TypedSpec().Running = true
				etcdService.TypedSpec().Healthy = true

				suite.Require().NoError(suite.State().Create(suite.Ctx(), etcdService))
			},
			AfterTearDown: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.State().Destroy(suite.Ctx(), v1alpha1.NewService("etcd").Metadata()))
			},
		},
	})
}
