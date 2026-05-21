// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//go:embed testdata/controllermanagerconfig.yaml
var expectedKubeControllerManagerConfigDocument []byte

func TestKubeControllerManagerConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeControllerManagerConfigV1Alpha1()
	cfg.PodImage = constants.KubernetesControllerManagerImage + ":v1.36.0"
	cfg.PodArgs = meta.Args{
		"feature-gates": meta.NewArgValue("AllBeta=true", nil),
	}
	cfg.PodEnv = map[string]string{
		"HTTPS_PROXY": "http://proxy:8080",
	}
	cfg.PodResources = k8s.ResourcesConfig{
		Requests: meta.Unstructured{
			Object: map[string]any{
				"cpu":    2,
				"memory": "2Gi",
			},
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeControllerManagerConfigDocument, marshaled)
}

func TestKubeControllerManagerConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeControllerManagerConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeControllerManagerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeControllerManagerConfig,
		},
		PodImage: constants.KubernetesControllerManagerImage + ":v1.36.0",
		PodArgs: meta.Args{
			"feature-gates": meta.NewArgValue("AllBeta=true", nil),
		},
		PodEnv: map[string]string{
			"HTTPS_PROXY": "http://proxy:8080",
		},
		PodResources: k8s.ResourcesConfig{
			Requests: meta.Unstructured{
				Object: map[string]any{
					"cpu":    2,
					"memory": "2Gi",
				},
			},
		},
	}, docs[0])
}

//nolint:dupl
func TestKubeControllerManagerConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		cfg           func() *k8s.KubeControllerManagerConfigV1Alpha1
		onMachineMode bool

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeControllerManagerConfigV1Alpha1,

			expectedError: "kube-controller-manager image cannot be empty",
		},
		{
			name: "disabled",
			cfg: func() *k8s.KubeControllerManagerConfigV1Alpha1 {
				cfg := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cfg.PodEnabled = new(false)

				return cfg
			},
		},
		{
			name: "invalid image, !local",
			cfg: func() *k8s.KubeControllerManagerConfigV1Alpha1 {
				cfg := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cfg.PodImage = "invalid-image"

				return cfg
			},
		},
		{
			name: "invalid image, local",
			cfg: func() *k8s.KubeControllerManagerConfigV1Alpha1 {
				cfg := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cfg.PodImage = "invalid-image"

				return cfg
			},
			onMachineMode: true,

			expectedError: `kube-controller-manager image is not valid: failed to parse Kubernetes version from image reference "invalid-image": invalid image reference: "invalid-image"`,
		},
		{
			name: "invalid resources",
			cfg: func() *k8s.KubeControllerManagerConfigV1Alpha1 {
				cfg := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cfg.PodImage = constants.KubernetesControllerManagerImage + ":v1.35.3"
				cfg.PodResources = k8s.ResourcesConfig{
					Requests: meta.Unstructured{
						Object: map[string]any{
							"invalid": "1",
						},
					},
				}

				return cfg
			},

			expectedError: `unsupported pod resource "invalid"`,
		},
		{
			name: "valid image, local",
			cfg: func() *k8s.KubeControllerManagerConfigV1Alpha1 {
				cfg := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cfg.PodImage = constants.KubernetesControllerManagerImage + ":v" + constants.DefaultKubernetesVersion

				return cfg
			},
			onMachineMode: true,
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeControllerManagerConfigV1Alpha1 {
				cfg := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cfg.PodImage = constants.KubernetesControllerManagerImage + ":v" + constants.DefaultKubernetesVersion
				cfg.PodArgs = meta.Args{
					"feature-gates": meta.NewArgValue("AllBeta=true", nil),
				}
				cfg.PodEnv = map[string]string{
					"HTTP_PROXY": "http://proxy:8080",
				}
				cfg.PodResources = k8s.ResourcesConfig{
					Requests: meta.Unstructured{
						Object: map[string]any{
							"cpu":    1,
							"memory": "1Gi",
						},
					},
					Limits: meta.Unstructured{
						Object: map[string]any{
							"cpu":    2,
							"memory": "2500Mi",
						},
					},
				}

				return cfg
			},
			onMachineMode: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var validationOptions []validation.Option

			if !test.onMachineMode {
				validationOptions = append(validationOptions, validation.WithLocal())
			}

			warnings, err := test.cfg().Validate(validationMode{}, validationOptions...)

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//nolint:dupl
func TestKubeControllerManagerConfigV1Alpha1Validate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
		},
		{
			name: "v1alpha1 with cluster ControllerManager config set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{}, //nolint:staticcheck // testing deprecated field
				},
			},

			expectedError: "kube-controller-manager config is already set in v1alpha1 config (.cluster.controllerManager)",
		},
		{
			name: "v1alpha1 with machine control plane ControllerManager config set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineControlPlane: &v1alpha1.MachineControlPlaneConfig{
						MachineControllerManager: &v1alpha1.MachineControllerManagerConfig{}, //nolint:staticcheck // testing deprecated field
					},
				},
			},

			expectedError: "kube-controller-manager config is already set in v1alpha1 config (.machine.controlplane.controllerManager)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeControllerManagerConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
