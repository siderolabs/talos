// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type StaticPodConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *StaticPodConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.StaticPodConfigController{}))

	suite.startRuntime()
}

func (suite *StaticPodConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *StaticPodConfigSuite) assertResource(
	md resource.Metadata,
	check func(res resource.Resource) error,
) func() error {
	return func() error {
		r, err := suite.state.Get(suite.ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		return check(r)
	}
}

func (suite *StaticPodConfigSuite) assertNoResource(md resource.Metadata) func() error {
	return func() error {
		_, err := suite.state.Get(suite.ctx, md)
		if err == nil {
			return retry.ExpectedErrorf("resource %s still exists", md)
		}

		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}
}

func (suite *StaticPodConfigSuite) TestReconcile() {
	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachinePods: []v1alpha1.Unstructured{
						{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "pod",
								"metadata": map[string]any{
									"name": "nginx",
								},
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "nginx",
											"image": "nginx",
										},
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		))

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(
				*k8s.NewStaticPod(k8s.NamespaceName, "default-nginx").Metadata(),
				func(res resource.Resource) error {
					v, ok, err := unstructured.NestedString(res.(*k8s.StaticPod).TypedSpec().Pod, "kind")
					suite.Require().NoError(err)
					suite.Assert().True(ok)
					suite.Assert().Equal("pod", v)

					return nil
				},
			),
		),
	)

	// update the pod changing the namespace
	cfg.Container().RawV1Alpha1().MachineConfig.MachinePods[0].Object["metadata"].(map[string]any)["namespace"] = "custom"
	suite.Require().NoError(suite.state.Update(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(
				*k8s.NewStaticPod(k8s.NamespaceName, "default-nginx").Metadata(),
			),
		),
	)
	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(
				*k8s.NewStaticPod(k8s.NamespaceName, "custom-nginx").Metadata(),
				func(res resource.Resource) error {
					v, ok, err := unstructured.NestedString(
						res.(*k8s.StaticPod).TypedSpec().Pod,
						"metadata",
						"namespace",
					)
					suite.Require().NoError(err)
					suite.Assert().True(ok)
					suite.Assert().Equal("custom", v)

					return nil
				},
			),
		),
	)

	// remove all pods
	cfg.Container().RawV1Alpha1().MachineConfig.MachinePods = nil
	suite.Require().NoError(suite.state.Update(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertNoResource(
				*k8s.NewStaticPod(k8s.NamespaceName, "custom-nginx").Metadata(),
			),
		),
	)
}

func (suite *StaticPodConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestStaticPodConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(StaticPodConfigSuite))
}
