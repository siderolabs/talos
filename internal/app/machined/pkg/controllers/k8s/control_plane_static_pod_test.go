// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type ControlPlaneStaticPodSuite struct {
	ctest.DefaultSuite
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileDefaults() {
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig()
	configControllerManager := k8s.NewControllerManagerConfig()
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig()
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
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileExtraMounts() {
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig()
	*configAPIServer.TypedSpec() = k8s.APIServerConfigSpec{
		ExtraVolumes: []k8s.ExtraVolume{
			{
				Name:      "foo",
				HostPath:  "/var/lib",
				MountPath: "/var/foo",
				ReadOnly:  true,
			},
		},
	}

	configControllerManager := k8s.NewControllerManagerConfig()
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig()
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

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), k8s.APIServerID, func(staticPod *k8s.StaticPod, assert *assert.Assertions) {
		apiServerPod, err := k8sadapter.StaticPod(staticPod).Pod()
		suite.Require().NoError(err)

		suite.Assert().Len(apiServerPod.Spec.Volumes, 4)
		suite.Assert().Len(apiServerPod.Spec.Containers[0].VolumeMounts, 4)

		suite.Assert().Equal([]v1.Volume{
			{
				Name: "secrets",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: constants.KubernetesAPIServerSecretsDir,
					},
				},
			},
			{
				Name: "config",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: constants.KubernetesAPIServerConfigDir,
					},
				},
			},
			{
				Name: "audit",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: constants.KubernetesAuditLogDir,
					},
				},
			},
			{
				Name: "foo",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/var/lib",
					},
				},
			},
		},
			apiServerPod.Spec.Volumes,
		)

		suite.Assert().Equal([]v1.VolumeMount{
			{
				Name:      "secrets",
				MountPath: constants.KubernetesAPIServerSecretsDir,
				ReadOnly:  true,
			},
			{
				Name:      "config",
				MountPath: constants.KubernetesAPIServerConfigDir,
				ReadOnly:  true,
			},
			{
				Name:      "audit",
				MountPath: constants.KubernetesAuditLogDir,
				ReadOnly:  false,
			},
			{
				Name:      "foo",
				MountPath: "/var/foo",
				ReadOnly:  true,
			},
		},
			apiServerPod.Spec.Containers[0].VolumeMounts,
		)
	})
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileExtraArgsK8s() {
	tests := []struct {
		k8sVersion  string
		args        map[string]k8s.ArgValues
		expected    map[string][]string
		expectError bool
	}{
		{
			k8sVersion: "v1.28.0", // authorization-config not supported and `authorization-mode` is not set
			args: map[string]k8s.ArgValues{
				"enable-admission-plugins": {Values: []string{"NodeRestriction,PodNodeSelector"}},
				"bind-address":             {Values: []string{"127.0.0.1"}},
				"audit-log-batch-max-size": {Values: []string{"2"}},
				"feature-gates":            {Values: []string{"PodNodeSelector=true"}},
			},
			expected: map[string][]string{
				"enable-admission-plugins": {"NodeRestriction,PodNodeSelector"},
				"authorization-mode":       {"Node,RBAC"},
				"bind-address":             {"127.0.0.1"},
				"audit-log-batch-max-size": {"2"},
				"feature-gates":            {"PodNodeSelector=true"},
			},
		},
		{
			k8sVersion: "v1.28.0", // authorization-config not supported
			args: map[string]k8s.ArgValues{
				"enable-admission-plugins": {Values: []string{"NodeRestriction,PodNodeSelector"}},
				"authorization-mode":       {Values: []string{"Webhook"}},
				"bind-address":             {Values: []string{"127.0.0.1"}},
				"audit-log-batch-max-size": {Values: []string{"2"}},
				"feature-gates":            {Values: []string{"PodNodeSelector=true"}},
			},
			expected: map[string][]string{
				"enable-admission-plugins": {"NodeRestriction,PodNodeSelector"},
				"authorization-mode":       {"Node,RBAC,Webhook"},
				"bind-address":             {"127.0.0.1"},
				"audit-log-batch-max-size": {"2"},
				"feature-gates":            {"PodNodeSelector=true"},
			},
		},
		{
			k8sVersion: "v1.29.0", // authorization-config supported, but feature-gates is alpha
			args: map[string]k8s.ArgValues{
				"enable-admission-plugins": {Values: []string{"NodeRestriction,PodNodeSelector"}},
				"bind-address":             {Values: []string{"127.0.0.1"}},
				"audit-log-batch-max-size": {Values: []string{"2"}},
				"feature-gates":            {Values: []string{"PodNodeSelector=true"}},
			},
			expected: map[string][]string{
				"enable-admission-plugins": {"NodeRestriction,PodNodeSelector"},
				"bind-address":             {"127.0.0.1"},
				"audit-log-batch-max-size": {"2"},
				"feature-gates":            {"StructuredAuthorizationConfiguration=true,PodNodeSelector=true"},
				"authorization-config":     {filepath.Join(constants.KubernetesAPIServerConfigDir, "authorization-config.yaml")},
			},
		},
		{
			k8sVersion: "v1.29.0", // authorization-config supported, but feature-gates is alpha, upgrade scenario where `authorization-mode` is already set
			args: map[string]k8s.ArgValues{
				"enable-admission-plugins": {Values: []string{"NodeRestriction,PodNodeSelector"}},
				"bind-address":             {Values: []string{"127.0.0.1"}},
				"audit-log-batch-max-size": {Values: []string{"2"}},
				"feature-gates":            {Values: []string{"PodNodeSelector=true"}},
				"authorization-mode":       {Values: []string{"Webhook,Node"}},
			},
			expected: map[string][]string{
				"enable-admission-plugins": {"NodeRestriction,PodNodeSelector"},
				"bind-address":             {"127.0.0.1"},
				"audit-log-batch-max-size": {"2"},
				"feature-gates":            {"PodNodeSelector=true"},
				"authorization-mode":       {"Node,RBAC,Webhook"},
			},
		},
		{
			k8sVersion: "v1.30.0", // authorization-config supported, feature-gates is beta (enabled by default), upgrade scenario where `authorization-webhook-*` is already set
			args: map[string]k8s.ArgValues{
				"enable-admission-plugins":      {Values: []string{"NodeRestriction,PodNodeSelector"}},
				"bind-address":                  {Values: []string{"127.0.0.1"}},
				"audit-log-batch-max-size":      {Values: []string{"2"}},
				"feature-gates":                 {Values: []string{"PodNodeSelector=true"}},
				"authorization-webhook-version": {Values: []string{"v1"}},
			},
			expected: map[string][]string{
				"enable-admission-plugins":      {"NodeRestriction,PodNodeSelector"},
				"bind-address":                  {"127.0.0.1"},
				"audit-log-batch-max-size":      {"2"},
				"feature-gates":                 {"PodNodeSelector=true"},
				"authorization-mode":            {"Node,RBAC"},
				"authorization-webhook-version": {"v1"},
			},
		},
		{
			k8sVersion: "v1.30.0", // authorization-config supported, feature-gates is beta (enabled by default)
			args: map[string]k8s.ArgValues{
				"enable-admission-plugins": {Values: []string{"NodeRestriction,PodNodeSelector"}},
				"bind-address":             {Values: []string{"127.0.0.1"}},
				"audit-log-batch-max-size": {Values: []string{"2"}},
				"feature-gates":            {Values: []string{"PodNodeSelector=true"}},
			},
			expected: map[string][]string{
				"enable-admission-plugins": {"NodeRestriction,PodNodeSelector"},
				"bind-address":             {"127.0.0.1"},
				"audit-log-batch-max-size": {"2"},
				"feature-gates":            {"PodNodeSelector=true"},
				"authorization-config":     {filepath.Join(constants.KubernetesAPIServerConfigDir, "authorization-config.yaml")},
			},
		},
		{
			args: map[string]k8s.ArgValues{
				"proxy-client-key-file": {Values: []string{"front-proxy-client.key"}},
			},
			expectError: true,
		},
	}

	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig()

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), secretStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configAPIServer))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), k8s.APIServerID, func(staticPod *k8s.StaticPod, assert *assert.Assertions) {})

	for _, test := range tests {
		configAPIServer.TypedSpec().ExtraArgs = test.args

		if test.k8sVersion != "" {
			configAPIServer.TypedSpec().Image = fmt.Sprintf("k8s.gcr.io/kube-apiserver:%s", test.k8sVersion)
		}

		oldData := configAPIServer.TypedSpec().ExtraArgs

		suite.Require().NoError(suite.State().Update(suite.Ctx(), configAPIServer))

		if test.expectError {
			// wait for some time to ensure that controller has picked the input
			time.Sleep(500 * time.Millisecond)

			// if the test expects an error, we should not have updated the extra args
			suite.Assert().Equal(oldData, configAPIServer.TypedSpec().ExtraArgs)

			continue
		}

		rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), k8s.APIServerID, func(staticPod *k8s.StaticPod, assert *assert.Assertions) {
			apiServerPod, err := k8sadapter.StaticPod(staticPod).Pod()
			suite.Require().NoError(err)

			assert.NotEmpty(apiServerPod.Spec.Containers)

			assertArg := func(arg string, equals []string) {
				actual := make([]string, 0, len(equals))

				for _, param := range apiServerPod.Spec.Containers[0].Command {
					if strings.HasPrefix(param, fmt.Sprintf("--%s", arg)) {
						key, value, ok := strings.Cut(param, "=")
						assert.True(ok, "expected '=' in %s", param)

						assert.Equal("--"+arg, key)

						actual = append(actual, value)
					}
				}

				assert.Equal(len(equals), len(actual))
				assert.ElementsMatch(equals, actual)
			}

			for k, v := range test.expected {
				assertArg(k, v)
			}
		})
	}
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileEnvironmentVariables() {
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig()

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), secretStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), configAPIServer))

	tests := []struct {
		env      map[string]string
		expected []v1.EnvVar
	}{
		{
			env: nil,
			expected: []v1.EnvVar{
				{
					Name: "POD_IP",
					ValueFrom: &v1.EnvVarSource{
						FieldRef: &v1.ObjectFieldSelector{
							FieldPath: "status.podIP",
						},
					},
				},
			},
		},
		{
			env: map[string]string{
				"foo": "bar",
				"baz": "$(foo)",
			},
			expected: []v1.EnvVar{
				{
					Name: "POD_IP",
					ValueFrom: &v1.EnvVarSource{
						FieldRef: &v1.ObjectFieldSelector{
							FieldPath: "status.podIP",
						},
					},
				},
				{
					Name:  "baz",
					Value: "$$(foo)",
				},
				{
					Name:  "foo",
					Value: "bar",
				},
			},
		},
	}

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), k8s.APIServerID, func(staticPod *k8s.StaticPod, assert *assert.Assertions) {})

	for _, test := range tests {
		configAPIServer.TypedSpec().EnvironmentVariables = test.env

		suite.Require().NoError(suite.State().Update(suite.Ctx(), configAPIServer))

		rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), k8s.APIServerID, func(staticPod *k8s.StaticPod, assert *assert.Assertions) {
			apiServerPod, err := k8sadapter.StaticPod(staticPod).Pod()
			suite.Require().NoError(err)

			assert.ElementsMatch(test.expected, apiServerPod.Spec.Containers[0].Env)
		})
	}
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileAdvertisedAddressArg() {
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configStatus))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), secretStatus))

	configAPIServer := k8s.NewAPIServerConfig()

	configAPIServer.TypedSpec().AdvertisedAddress = "$(POD_IP)"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configAPIServer))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), k8s.APIServerID, func(staticPod *k8s.StaticPod, assert *assert.Assertions) {
		apiServerPod, err := k8sadapter.StaticPod(staticPod).Pod()
		suite.Require().NoError(err)

		assert.NotEmpty(apiServerPod.Spec.Containers)

		assert.Contains(apiServerPod.Spec.Containers[0].Command, "--advertise-address=$(POD_IP)")
	})

	configAPIServer.TypedSpec().AdvertisedAddress = ""

	suite.Assert().NoError(suite.State().Update(suite.Ctx(), configAPIServer))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), k8s.APIServerID, func(staticPod *k8s.StaticPod, assert *assert.Assertions) {
		apiServerPod, err := k8sadapter.StaticPod(staticPod).Pod()
		suite.Require().NoError(err)

		assert.NotEmpty(apiServerPod.Spec.Containers)

		assert.NotContains(apiServerPod.Spec.Containers[0].Command, "--advertise-address")
	})
}

func (suite *ControlPlaneStaticPodSuite) TestControlPlaneStaticPodsExceptScheduler() {
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig()
	configControllerManager := k8s.NewControllerManagerConfig()
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig()
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

	configAPIServer := k8s.NewAPIServerConfig()
	configControllerManager := k8s.NewControllerManagerConfig()
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig()
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
				Value: strconv.FormatInt(1024*1024*1024*k8sctrl.GoGCMemLimitPercentage/100, 10),
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
