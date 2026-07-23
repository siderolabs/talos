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

//go:embed testdata/proxyconfig.yaml
var expectedKubeProxyConfigDocument []byte

func TestKubeProxyConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeProxyConfigV1Alpha1()
	cfg.ProxyImage = constants.KubeProxyImage + ":v1.35.3"
	cfg.ProxyMode = "nftables"
	cfg.ProxyConfig = meta.Unstructured{
		Object: map[string]any{
			"bindAddressHardFail": true,
		},
	}
	cfg.ProxyExtraArgs = meta.Args{
		"proxy-mode": meta.NewArgValue("nftables", nil),
	}
	cfg.ProxyEnabled = new(true)
	cfg.ProxyResources = k8s.ResourcesConfig{
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

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeProxyConfigDocument, marshaled)
}

func TestKubeProxyConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeProxyConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeProxyConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeProxyConfig,
		},
		ProxyImage: constants.KubeProxyImage + ":v1.35.3",
		ProxyMode:  "nftables",
		ProxyConfig: meta.Unstructured{
			Object: map[string]any{
				"bindAddressHardFail": true,
			},
		},
		ProxyExtraArgs: meta.Args{
			"proxy-mode": meta.NewArgValue("nftables", nil),
		},
		ProxyEnabled: new(true),
		ProxyResources: k8s.ResourcesConfig{
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
		},
	}, docs[0])
}

func TestKubeProxyConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		cfg           func() *k8s.KubeProxyConfigV1Alpha1
		onMachineMode bool

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeProxyConfigV1Alpha1,

			expectedError: "proxy image cannot be empty",
		},
		{
			name: "disabled",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyEnabled = new(false)

				return cfg
			},
		},
		{
			name: "invalid image, !local",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = "invalid-image"

				return cfg
			},
		},
		{
			name: "invalid image, local",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = "invalid-image"

				return cfg
			},
			onMachineMode: true,

			expectedError: `proxy image is not valid: failed to parse Kubernetes version from image reference "invalid-image": invalid image reference: "invalid-image"`,
		},
		{
			name: "invalid mode",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = constants.KubeProxyImage + ":v" + constants.DefaultKubernetesVersion
				cfg.ProxyMode = "bogus"

				return cfg
			},

			expectedError: `invalid proxy mode "bogus": supported modes are iptables, ipvs and nftables`,
		},
		{
			name: "not recommended mode",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = constants.KubeProxyImage + ":v" + constants.DefaultKubernetesVersion
				cfg.ProxyMode = "ipvs"

				return cfg
			},

			expectedWarnings: []string{`proxy mode "ipvs" is not recommended, please switch to nftables if possible`},
		},
		{
			name: "extra args warning",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = constants.KubeProxyImage + ":v" + constants.DefaultKubernetesVersion
				cfg.ProxyExtraArgs = meta.Args{
					"proxy-mode": meta.NewArgValue("nftables", nil),
				}

				return cfg
			},

			expectedWarnings: []string{"extra arguments for kube-proxy may not work as expected, please use configuration instead"},
		},
		{
			name: "invalid resources",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = constants.KubeProxyImage + ":v1.35.3"
				cfg.ProxyResources = k8s.ResourcesConfig{
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
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = constants.KubeProxyImage + ":v" + constants.DefaultKubernetesVersion

				return cfg
			},
			onMachineMode: true,
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeProxyConfigV1Alpha1 {
				cfg := k8s.NewKubeProxyConfigV1Alpha1()
				cfg.ProxyImage = constants.KubeProxyImage + ":v" + constants.DefaultKubernetesVersion
				cfg.ProxyMode = "nftables"
				cfg.ProxyConfig = meta.Unstructured{
					Object: map[string]any{
						"bindAddressHardFail": true,
					},
				}
				cfg.ProxyResources = k8s.ResourcesConfig{
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
func TestKubeProxyConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with cluster proxy config set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ProxyConfig: &v1alpha1.ProxyConfig{}, //nolint:staticcheck // testing deprecated field
				},
			},

			expectedError: "cluster proxy config in v1alpha1 config (.machine.cluster.proxy) can't be used with KubeProxyConfig document, please remove it to avoid conflicts",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeProxyConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
