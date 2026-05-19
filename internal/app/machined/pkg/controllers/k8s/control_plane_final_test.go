// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
