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
)

//go:embed testdata/authorizerconfig.yaml
var expectedKubeAuthorizerConfigDocument []byte

func TestKubeAuthorizerConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
	cfg.MetaName = "webhook"
	cfg.AuthorizerType = "Webhook"
	cfg.AuthorizerWebhook.Object = map[string]any{
		"timeout":                    "3s",
		"subjectAccessReviewVersion": "v1",
		"failurePolicy":              "NoOpinion",
	}
	cfg.AuthorizerWebhookKubeconfig = testWebhookKubeconfig

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAuthorizerConfigDocument, marshaled)
}

func TestKubeAuthorizerConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeAuthorizerConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeAuthorizerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeAuthorizerConfig,
		},
		MetaName:       "webhook",
		AuthorizerType: "Webhook",
		AuthorizerWebhook: meta.Unstructured{
			Object: map[string]any{
				"timeout":                    "3s",
				"subjectAccessReviewVersion": "v1",
				"failurePolicy":              "NoOpinion",
			},
		},
		AuthorizerWebhookKubeconfig: testWebhookKubeconfig,
	}, docs[0])
}

func TestKubeAuthorizerConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
	cfg.MetaName = "webhook"
	cfg.AuthorizerType = "Webhook"
	cfg.AuthorizerWebhookKubeconfig = testWebhookKubeconfig

	cfg.Redact("REDACTED")

	assert.Equal(t, "REDACTED", cfg.AuthorizerWebhookKubeconfig)

	cfg = k8s.NewKubeAuthorizerConfigV1Alpha1()
	cfg.Redact("REDACTED")

	assert.Empty(t, cfg.AuthorizerWebhookKubeconfig)
}

func TestKubeAuthorizerConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeAuthorizerConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeAuthorizerConfigV1Alpha1,

			expectedError: "authorizer name is required\nauthorizer type is required\nauthorizer type  is not allowed, allowed types are [Node RBAC Webhook]",
		},
		{
			name: "invalid type",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "custom"
				cfg.AuthorizerType = "Custom"

				return cfg
			},

			expectedError: "authorizer type Custom is not allowed, allowed types are [Node RBAC Webhook]",
		},
		{
			name: "webhook without configuration",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "webhook"
				cfg.AuthorizerType = "Webhook"

				return cfg
			},

			expectedError: "authorizer webhook configuration is required for Webhook authorizer type",
		},
		{
			name: "invalid name",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "My_Authorizer"
				cfg.AuthorizerType = "Node"

				return cfg
			},

			expectedError: "authorizer name is invalid: domain doesn't match required format: \"My_Authorizer\"",
		},
		{
			name: "webhook without connection info",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "webhook"
				cfg.AuthorizerType = "Webhook"
				cfg.AuthorizerWebhook.Object = map[string]any{
					"timeout": "3s",
				}

				return cfg
			},

			expectedError: "authorizer kubeconfig is required for Webhook authorizer type",
		},
		{
			name: "webhook with kubeconfig file",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "webhook"
				cfg.AuthorizerType = "Webhook"
				cfg.AuthorizerWebhook.Object = map[string]any{
					"connectionInfo": map[string]any{
						"type":           "KubeConfigFile",
						"kubeConfigFile": "/etc/kubernetes/webhook.kubeconfig",
					},
				}

				return cfg
			},

			expectedError: "authorizer webhook connectionInfo type KubeConfigFile is not supported, as kube-apiserver can't access host files: use kubeconfig instead",
		},
		{
			name: "webhook with kubeconfig and connection info",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "webhook"
				cfg.AuthorizerType = "Webhook"
				cfg.AuthorizerWebhook.Object = map[string]any{
					"connectionInfo": map[string]any{
						"type": "InClusterConfig",
					},
				}
				cfg.AuthorizerWebhookKubeconfig = testWebhookKubeconfig

				return cfg
			},

			expectedError: "authorizer webhook connectionInfo should not be set when kubeconfig is set, Talos fills it in",
		},
		{
			name: "webhook with invalid kubeconfig",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "webhook"
				cfg.AuthorizerType = "Webhook"
				cfg.AuthorizerWebhook.Object = map[string]any{
					"timeout": "3s",
				}
				cfg.AuthorizerWebhookKubeconfig = "apiVersion: v1\nkind: Config\n"

				return cfg
			},

			expectedError: "authorizer kubeconfig: kubeconfig current-context is required",
		},
		{
			name: "kubeconfig on non-webhook type",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "rbac"
				cfg.AuthorizerType = "RBAC"
				cfg.AuthorizerWebhookKubeconfig = testWebhookKubeconfig

				return cfg
			},

			expectedError: "authorizer kubeconfig is only allowed for Webhook authorizer type",
		},
		{
			name: "webhook configuration on non-webhook type",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "rbac"
				cfg.AuthorizerType = "RBAC"
				cfg.AuthorizerWebhook.Object = map[string]any{
					"timeout": "3s",
				}

				return cfg
			},

			expectedWarnings: []string{"authorizer webhook configuration is not allowed non-Webhook authorizer types"},
		},
		{
			name: "valid node",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "node"
				cfg.AuthorizerType = "Node"

				return cfg
			},
		},
		{
			name: "valid webhook",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "webhook"
				cfg.AuthorizerType = "Webhook"
				cfg.AuthorizerWebhook.Object = map[string]any{
					"timeout": "3s",
				}
				cfg.AuthorizerWebhookKubeconfig = testWebhookKubeconfig

				return cfg
			},
		},
		{
			name: "valid in-cluster webhook",
			cfg: func() *k8s.KubeAuthorizerConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthorizerConfigV1Alpha1()
				cfg.MetaName = "webhook.example.com"
				cfg.AuthorizerType = "Webhook"
				cfg.AuthorizerWebhook.Object = map[string]any{
					"connectionInfo": map[string]any{
						"type": "InClusterConfig",
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

func TestKubeAuthorizerConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with admission control config set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					APIServerConfig: &v1alpha1.APIServerConfig{ //nolint:staticcheck // testing deprecated field
						AdmissionControlConfig: v1alpha1.AdmissionPluginConfigList{ //nolint:staticcheck // testing deprecated field
							{
								PluginName: "PodSecurity",
							},
						},
					},
				},
			},

			expectedError: "admission control plugin config is already set in v1alpha1 config",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeAuthorizerConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
