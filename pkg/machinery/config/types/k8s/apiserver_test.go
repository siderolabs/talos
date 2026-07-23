// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
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

//go:embed testdata/apiserverconfig.yaml
var expectedKubeAPIServerConfigDocument []byte

func TestKubeAPIServerConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAPIServerConfigV1Alpha1()
	cfg.PodImage = constants.KubernetesAPIServerImage + ":v1.36.0"
	cfg.PodArgs = meta.Args{
		"feature-gates": meta.NewArgValue("ServerSideApply=true", nil),
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
	cfg.PodAPIPort = new(8443)
	cfg.PodCertExtraSANs = []string{"k8s.example.com"}
	cfg.PodStartupProbes = new(false)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAPIServerConfigDocument, marshaled)
}

func TestKubeAPIServerConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeAPIServerConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeAPIServerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeAPIServerConfig,
		},
		PodImage: constants.KubernetesAPIServerImage + ":v1.36.0",
		PodArgs: meta.Args{
			"feature-gates": meta.NewArgValue("ServerSideApply=true", nil),
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
		PodAPIPort:       new(8443),
		PodCertExtraSANs: []string{"k8s.example.com"},
		PodStartupProbes: new(false),
	}, docs[0])
}

//nolint:dupl
func TestKubeAPIServerConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		cfg           func() *k8s.KubeAPIServerConfigV1Alpha1
		onMachineMode bool

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeAPIServerConfigV1Alpha1,

			expectedError: "kube-apiserver image cannot be empty",
		},
		{
			name: "invalid image, !local",
			cfg: func() *k8s.KubeAPIServerConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerConfigV1Alpha1()
				cfg.PodImage = "invalid-image"

				return cfg
			},
		},
		{
			name: "invalid image, local",
			cfg: func() *k8s.KubeAPIServerConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerConfigV1Alpha1()
				cfg.PodImage = "invalid-image"

				return cfg
			},
			onMachineMode: true,

			expectedError: `kube-apiserver image is not valid: failed to parse Kubernetes version from image reference "invalid-image": invalid image reference: "invalid-image"`,
		},
		{
			name: "invalid resources",
			cfg: func() *k8s.KubeAPIServerConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerConfigV1Alpha1()
				cfg.PodImage = constants.KubernetesAPIServerImage + ":v1.35.3"
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
			name: "invalid args",
			cfg: func() *k8s.KubeAPIServerConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerConfigV1Alpha1()
				cfg.PodImage = constants.KubernetesAPIServerImage + ":v1.35.3"
				cfg.PodArgs = meta.Args{
					"authorization-mode": meta.NewArgValue("RBAC", nil),
					"anonymous-auth":     meta.NewArgValue("false", nil),
				}

				return cfg
			},

			expectedError: "kube-apiserver extra argument \"anonymous-auth\" is not allowed: use KubeAuthenticationConfig\n" +
				"kube-apiserver extra argument \"authorization-mode\" is not allowed: use KubeAuthorizationConfig",
		},
		{
			name: "valid image, local",
			cfg: func() *k8s.KubeAPIServerConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerConfigV1Alpha1()
				cfg.PodImage = constants.KubernetesAPIServerImage + ":v" + constants.DefaultKubernetesVersion

				return cfg
			},
			onMachineMode: true,
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeAPIServerConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerConfigV1Alpha1()
				cfg.PodImage = constants.KubernetesAPIServerImage + ":v" + constants.DefaultKubernetesVersion
				cfg.PodArgs = meta.Args{
					"feature-gates": meta.NewArgValue("ServerSideApply=true", nil),
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

func TestKubeAPIServerConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with cluster APIServer config set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					APIServerConfig: &v1alpha1.APIServerConfig{}, //nolint:staticcheck // testing deprecated field
				},
			},

			expectedError: "kube-apiserver config is already set in v1alpha1 config (.cluster.apiServer)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeAPIServerConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
