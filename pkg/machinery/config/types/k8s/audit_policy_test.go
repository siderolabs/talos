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

//go:embed testdata/auditpolicyconfig.yaml
var expectedKubeAuditPolicyConfigDocument []byte

func TestKubeAuditPolicyConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAuditPolicyConfigV1Alpha1()
	cfg.AuditConfig.Object = map[string]any{
		"apiVersion": "audit.k8s.io/v1",
		"kind":       "Policy",
		"rules": []any{
			map[string]any{
				"level": "Metadata",
			},
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAuditPolicyConfigDocument, marshaled)
}

func TestKubeAuditPolicyConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeAuditPolicyConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeAuditPolicyConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeAuditPolicyConfig,
		},
		AuditConfig: meta.Unstructured{
			Object: map[string]any{
				"apiVersion": "audit.k8s.io/v1",
				"kind":       "Policy",
				"rules": []any{
					map[string]any{
						"level": "Metadata",
					},
				},
			},
		},
	}, docs[0])
}

func TestKubeAuditPolicyConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with audit policy config set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					APIServerConfig: &v1alpha1.APIServerConfig{ //nolint:staticcheck // testing deprecated field
						AuditPolicyConfig: meta.Unstructured{
							Object: map[string]any{
								"apiVersion": "audit.k8s.io/v1",
								"kind":       "Policy",
							},
						},
					},
				},
			},

			expectedError: "audit policy config is already set in v1alpha1 config",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeAuditPolicyConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
