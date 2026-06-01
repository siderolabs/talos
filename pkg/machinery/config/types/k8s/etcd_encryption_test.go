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
)

//go:embed testdata/etcdencryptionconfig.yaml
var expectedKubeEtcdEncryptionConfigDocument []byte

func TestKubeEtcdEncryptionConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeEtcdEncryptionConfigV1Alpha1()
	cfg.Config.Object = map[string]any{
		"resources": []any{
			map[string]any{
				"providers": []any{
					map[string]any{
						"aescbc": map[string]any{
							"keys": []any{
								map[string]any{
									"name":   "key2",
									"secret": "w=",
								},
							},
						},
					},
					map[string]any{
						"identity": map[string]any{},
					},
				},
				"resources": []any{
					"secrets",
				},
			},
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeEtcdEncryptionConfigDocument, marshaled)
}

func TestKubeEtcdEncryptionConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeEtcdEncryptionConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeEtcdEncryptionConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeEtcdEncryptionConfig,
		},
		Config: meta.Unstructured{
			Object: map[string]any{
				"resources": []any{
					map[string]any{
						"providers": []any{
							map[string]any{
								"aescbc": map[string]any{
									"keys": []any{
										map[string]any{
											"name":   "key2",
											"secret": "w=",
										},
									},
								},
							},
							map[string]any{
								"identity": map[string]any{},
							},
						},
						"resources": []any{
							"secrets",
						},
					},
				},
			},
		},
	}, docs[0])
}

func TestKubeEtcdEncryptionConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeEtcdEncryptionConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeEtcdEncryptionConfigV1Alpha1,

			expectedError: "etcd encryption config is required",
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeEtcdEncryptionConfigV1Alpha1 {
				cfg := k8s.NewKubeEtcdEncryptionConfigV1Alpha1()
				cfg.Config.Object = map[string]any{
					"resources": []any{
						map[string]any{
							"resources": []any{
								"secrets",
							},
						},
					},
				}

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//go:embed testdata/etcdencryptionconfig_redacted.yaml
var expectedKubeEtcdEncryptionConfigRedactedDocument []byte

func TestKubeEtcdEncryptionConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeEtcdEncryptionConfigV1Alpha1()
	cfg.Config.Object = map[string]any{
		"resources": []any{
			map[string]any{
				"providers": []any{
					map[string]any{
						"aescbc": map[string]any{
							"keys": []any{
								map[string]any{
									"name":   "key2",
									"secret": "w=",
								},
							},
						},
					},
					map[string]any{
						"secretbox": map[string]any{
							"keys": []any{
								map[string]any{
									"name":   "key1",
									"secret": "M-EXAMPLE-SECRET-DO-NOT-USE-w=",
								},
								map[string]any{
									"name":   "key3",
									"secret": "another-secret",
								},
							},
						},
					},
				},
				"resources": []any{
					"secrets",
				},
			},
		},
	}

	cfg.Redact("REDACTED")

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeEtcdEncryptionConfigRedactedDocument, marshaled)
}

func TestKubeEtcdEncryptionConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with etcd secretbox encryption secret set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterSecretboxEncryptionSecret: "foo",
				},
			},

			expectedError: "etcd encryption config is already set in v1alpha1 config",
		},
		{
			name: "v1alpha1 with etcd aescbc encryption secret set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterAESCBCEncryptionSecret: "foo",
				},
			},

			expectedError: "etcd encryption config is already set in v1alpha1 config",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeEtcdEncryptionConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type validationMode struct{}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (validationMode) InContainer() bool {
	return false
}
