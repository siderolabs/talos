// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type StaticPodConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *StaticPodConfigSuite) TestReconcile() {
	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachinePods: []meta.Unstructured{
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
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, "default-nginx", func(r *k8s.StaticPod, asrt *assert.Assertions) {
		v, ok, err := unstructured.NestedString(r.TypedSpec().Pod, "kind")
		asrt.NoError(err)
		asrt.True(ok)
		asrt.Equal("pod", v)
	})

	// update the pod changing the namespace
	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachinePods[0].Object["metadata"].(map[string]any)["namespace"] = "custom"

		return nil
	})

	ctest.AssertNoResource[*k8s.StaticPod](suite, "default-nginx")
	ctest.AssertResource(suite, "custom-nginx", func(r *k8s.StaticPod, asrt *assert.Assertions) {
		v, ok, err := unstructured.NestedString(r.TypedSpec().Pod, "metadata", "namespace")
		asrt.NoError(err)
		asrt.True(ok)
		asrt.Equal("custom", v)
	})

	// remove all pods
	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachinePods = nil

		return nil
	})

	ctest.AssertNoResource[*k8s.StaticPod](suite, "custom-nginx")
}

func TestStaticPodConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StaticPodConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.StaticPodConfigController{}))
			},
		},
	})
}
