// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"fmt"
	"log"
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
	v1 "k8s.io/api/core/v1"

	k8sadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/k8s"
	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

type ControlPlaneStaticPodSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *ControlPlaneStaticPodSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.ControlPlaneStaticPodController{}))

	etcdService := v1alpha1.NewService("etcd")
	etcdService.TypedSpec().Running = true
	etcdService.TypedSpec().Healthy = true

	suite.Require().NoError(suite.state.Create(suite.ctx, etcdService))

	suite.startRuntime()
}

func (suite *ControlPlaneStaticPodSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

//nolint:dupl
func (suite *ControlPlaneStaticPodSuite) assertControlPlaneStaticPods(manifests []string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	ids := slices.Map(resources.Items, func(r resource.Resource) string { return r.Metadata().ID() })

	if !reflect.DeepEqual(manifests, ids) {
		return retry.ExpectedError(fmt.Errorf("expected %q, got %q", manifests, ids))
	}

	return nil
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileDefaults() {
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig()
	configControllerManager := k8s.NewControllerManagerConfig()
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig()
	configScheduler.TypedSpec().Enabled = true

	suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, secretStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, configAPIServer))
	suite.Require().NoError(suite.state.Create(suite.ctx, configControllerManager))
	suite.Require().NoError(suite.state.Create(suite.ctx, configScheduler))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertControlPlaneStaticPods(
					[]string{
						"kube-apiserver",
						"kube-controller-manager",
						"kube-scheduler",
					},
				)
			},
		),
	)

	// tear down etcd service
	suite.Require().NoError(suite.state.Destroy(suite.ctx, v1alpha1.NewService("etcd").Metadata()))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				list, err := suite.state.List(
					suite.ctx,
					resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "", resource.VersionUndefined),
				)
				if err != nil {
					return err
				}

				if len(list.Items) > 0 {
					return retry.ExpectedErrorf("expected no pods, got %d", len(list.Items))
				}

				return nil
			},
		),
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

	suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, secretStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, configAPIServer))
	suite.Require().NoError(suite.state.Create(suite.ctx, configControllerManager))
	suite.Require().NoError(suite.state.Create(suite.ctx, configScheduler))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertControlPlaneStaticPods(
					[]string{
						"kube-apiserver",
						"kube-controller-manager",
						"kube-scheduler",
					},
				)
			},
		),
	)

	r, err := suite.state.Get(
		suite.ctx,
		resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "kube-apiserver", resource.VersionUndefined),
	)
	suite.Require().NoError(err)

	apiServerPod, err := k8sadapter.StaticPod(r.(*k8s.StaticPod)).Pod()
	suite.Require().NoError(err)

	suite.Assert().Len(apiServerPod.Spec.Volumes, 4)
	suite.Assert().Len(apiServerPod.Spec.Containers[0].VolumeMounts, 4)

	suite.Assert().Equal(
		v1.Volume{
			Name: "secrets",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: constants.KubernetesAPIServerSecretsDir,
				},
			},
		}, apiServerPod.Spec.Volumes[0],
	)

	suite.Assert().Equal(
		v1.Volume{
			Name: "config",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: constants.KubernetesAPIServerConfigDir,
				},
			},
		}, apiServerPod.Spec.Volumes[1],
	)

	suite.Assert().Equal(
		v1.Volume{
			Name: "audit",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: constants.KubernetesAuditLogDir,
				},
			},
		}, apiServerPod.Spec.Volumes[2],
	)

	suite.Assert().Equal(
		v1.Volume{
			Name: "foo",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib",
				},
			},
		}, apiServerPod.Spec.Volumes[3],
	)

	suite.Assert().Equal(
		v1.VolumeMount{
			Name:      "secrets",
			MountPath: constants.KubernetesAPIServerSecretsDir,
			ReadOnly:  true,
		}, apiServerPod.Spec.Containers[0].VolumeMounts[0],
	)

	suite.Assert().Equal(
		v1.VolumeMount{
			Name:      "config",
			MountPath: constants.KubernetesAPIServerConfigDir,
			ReadOnly:  true,
		}, apiServerPod.Spec.Containers[0].VolumeMounts[1],
	)

	suite.Assert().Equal(
		v1.VolumeMount{
			Name:      "audit",
			MountPath: constants.KubernetesAuditLogDir,
			ReadOnly:  false,
		}, apiServerPod.Spec.Containers[0].VolumeMounts[2],
	)

	suite.Assert().Equal(
		v1.VolumeMount{
			Name:      "foo",
			MountPath: "/var/foo",
			ReadOnly:  true,
		}, apiServerPod.Spec.Containers[0].VolumeMounts[3],
	)
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileExtraArgs() {
	tests := []struct {
		args        map[string]string
		expected    map[string]string
		expectError bool
	}{
		{
			args: map[string]string{
				"enable-admission-plugins": "NodeRestriction,PodNodeSelector",
				"authorization-mode":       "Webhook",
				"bind-address":             "127.0.0.1",
				"audit-log-batch-max-size": "2",
			},
			expected: map[string]string{
				"enable-admission-plugins": "NodeRestriction,PodNodeSelector",
				"authorization-mode":       "Node,RBAC,Webhook",
				"bind-address":             "127.0.0.1",
				"audit-log-batch-max-size": "2",
			},
		},
		{
			args: map[string]string{
				"proxy-client-key-file": "front-proxy-client.key",
			},
			expectError: true,
		},
	}
	for _, test := range tests {
		configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
		secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
		configAPIServer := k8s.NewAPIServerConfig()

		*configAPIServer.TypedSpec() = k8s.APIServerConfigSpec{
			ExtraArgs: test.args,
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))
		suite.Require().NoError(suite.state.Create(suite.ctx, secretStatus))
		suite.Require().NoError(suite.state.Create(suite.ctx, configAPIServer))

		if test.expectError {
			// wait for some time to ensure that controller has picked the input
			time.Sleep(500 * time.Millisecond)

			_, err := suite.state.Get(
				suite.ctx,
				resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "kube-apiserver", resource.VersionUndefined),
			)
			suite.Require().Error(err)

			continue
		}

		suite.Assert().NoError(
			retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				func() error {
					return suite.assertControlPlaneStaticPods(
						[]string{
							"kube-apiserver",
						},
					)
				},
			),
		)

		r, err := suite.state.Get(
			suite.ctx,
			resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "kube-apiserver", resource.VersionUndefined),
		)
		suite.Require().NoError(err)

		apiServerPod, err := k8sadapter.StaticPod(r.(*k8s.StaticPod)).Pod()
		suite.Require().NoError(err)

		suite.Require().NotEmpty(apiServerPod.Spec.Containers)

		assertArg := func(arg, equals string) {
			for _, param := range apiServerPod.Spec.Containers[0].Command {
				if strings.HasPrefix(param, fmt.Sprintf("--%s", arg)) {
					parts := strings.Split(param, "=")

					suite.Require().Equal(equals, parts[1])
				}
			}
		}

		for k, v := range test.expected {
			assertArg(k, v)
		}

		suite.Require().NoError(suite.state.Destroy(suite.ctx, configStatus.Metadata()))
		suite.Require().NoError(suite.state.Destroy(suite.ctx, secretStatus.Metadata()))
		suite.Require().NoError(suite.state.Destroy(suite.ctx, configAPIServer.Metadata()))

		suite.Assert().NoError(
			retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				func() error {
					list, err := suite.state.List(
						suite.ctx,
						resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "", resource.VersionUndefined),
					)
					if err != nil {
						return err
					}

					if len(list.Items) > 0 {
						return retry.ExpectedErrorf("expected no pods, got %d", len(list.Items))
					}

					return nil
				},
			),
		)
	}
}

func (suite *ControlPlaneStaticPodSuite) TestReconcileEnvironmentVariables() {
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)

	suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, secretStatus))

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
	for _, test := range tests {
		configAPIServer := k8s.NewAPIServerConfig()

		*configAPIServer.TypedSpec() = k8s.APIServerConfigSpec{
			EnvironmentVariables: test.env,
		}

		suite.Require().NoError(suite.state.Create(suite.ctx, configAPIServer))

		suite.Assert().NoError(
			retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				func() error {
					return suite.assertControlPlaneStaticPods(
						[]string{
							"kube-apiserver",
						},
					)
				},
			),
		)

		r, err := suite.state.Get(
			suite.ctx,
			resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "kube-apiserver", resource.VersionUndefined),
		)
		suite.Require().NoError(err)

		apiServerPod, err := k8sadapter.StaticPod(r.(*k8s.StaticPod)).Pod()
		suite.Require().NoError(err)

		suite.Require().NotEmpty(apiServerPod.Spec.Containers)

		suite.Assert().Equal(test.expected, apiServerPod.Spec.Containers[0].Env)

		suite.Require().NoError(suite.state.Destroy(suite.ctx, configAPIServer.Metadata()))

		suite.Assert().NoError(
			retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				func() error {
					list, err := suite.state.List(
						suite.ctx,
						resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "", resource.VersionUndefined),
					)
					if err != nil {
						return err
					}

					if len(list.Items) > 0 {
						return retry.ExpectedErrorf("expected no pods, got %d", len(list.Items))
					}

					return nil
				},
			),
		)
	}
}

func (suite *ControlPlaneStaticPodSuite) TestControlPlaneStaticPodsExceptScheduler() {
	configStatus := k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID)
	secretStatus := k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID)
	configAPIServer := k8s.NewAPIServerConfig()
	configControllerManager := k8s.NewControllerManagerConfig()
	configControllerManager.TypedSpec().Enabled = true
	configScheduler := k8s.NewSchedulerConfig()
	configScheduler.TypedSpec().Enabled = true

	suite.Require().NoError(suite.state.Create(suite.ctx, configStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, secretStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, configAPIServer))
	suite.Require().NoError(suite.state.Create(suite.ctx, configControllerManager))
	suite.Require().NoError(suite.state.Create(suite.ctx, configScheduler))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertControlPlaneStaticPods(
					[]string{
						"kube-apiserver",
						"kube-controller-manager",
						"kube-scheduler",
					},
				)
			},
		),
	)

	// flip enabled to disable scheduler
	_, err := suite.state.UpdateWithConflicts(
		suite.ctx, configScheduler.Metadata(), func(r resource.Resource) error {
			r.(*k8s.SchedulerConfig).TypedSpec().Enabled = false

			return nil
		},
	)
	suite.Require().NoError(err)

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertControlPlaneStaticPods(
					[]string{
						"kube-apiserver",
						"kube-controller-manager",
					},
				)
			},
		),
	)
}

func (suite *ControlPlaneStaticPodSuite) TearDownTest() {
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
}

func TestControlPlaneStaticPodSuite(t *testing.T) {
	suite.Run(t, new(ControlPlaneStaticPodSuite))
}
