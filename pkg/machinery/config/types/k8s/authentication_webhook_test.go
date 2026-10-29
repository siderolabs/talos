// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"testing"
	"time"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

const testWebhookKubeconfig = `apiVersion: v1
kind: Config
clusters:
  - name: webhook
    cluster:
      server: https://127.0.0.1:21362/authenticate
      certificate-authority-data: Y2EK
users:
  - name: apiserver
    user:
      token: secret
contexts:
  - name: webhook
    context:
      cluster: webhook
      user: apiserver
current-context: webhook
`

//go:embed testdata/authenticationwebhookconfig.yaml
var expectedKubeAuthenticationWebhookConfigDocument []byte

func TestKubeAuthenticationWebhookConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
	cfg.WebhookKubeconfig = testWebhookKubeconfig
	cfg.WebhookVersion = "v1"
	cfg.WebhookCacheTTL = pointer.To(5 * time.Minute)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAuthenticationWebhookConfigDocument, marshaled)
}

func TestKubeAuthenticationWebhookConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeAuthenticationWebhookConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeAuthenticationWebhookConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeAuthenticationWebhookConfig,
		},
		WebhookKubeconfig: testWebhookKubeconfig,
		WebhookVersion:    "v1",
		WebhookCacheTTL:   pointer.To(5 * time.Minute),
	}, docs[0])

	webhookCfg := provider.K8sAuthenticationWebhookConfig()
	require.NotNil(t, webhookCfg)

	assert.Equal(t, testWebhookKubeconfig, webhookCfg.Kubeconfig())
	assert.Equal(t, "v1", webhookCfg.Version())
	assert.Equal(t, optional.Some(5*time.Minute), webhookCfg.CacheTTL())
}

func TestKubeAuthenticationWebhookConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
	cfg.WebhookKubeconfig = testWebhookKubeconfig
	cfg.WebhookVersion = "v1"

	cfg.Redact("REDACTED")

	assert.Equal(t, "REDACTED", cfg.WebhookKubeconfig)
	assert.Equal(t, "v1", cfg.WebhookVersion)
}

//nolint:dupl
func TestKubeAuthenticationWebhookConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeAuthenticationWebhookConfigV1Alpha1

		expectedError string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeAuthenticationWebhookConfigV1Alpha1,

			expectedError: "authentication webhook kubeconfig is required",
		},
		{
			name: "invalid version and cache TTL",
			cfg: func() *k8s.KubeAuthenticationWebhookConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
				cfg.WebhookKubeconfig = testWebhookKubeconfig
				cfg.WebhookVersion = "v2"
				cfg.WebhookCacheTTL = pointer.To(-time.Second)

				return cfg
			},

			expectedError: "authentication webhook version \"v2\" is not allowed, allowed versions are [v1 v1beta1]\n" +
				"authentication webhook cache TTL should not be negative",
		},
		{
			name: "kubeconfig not yaml",
			cfg: func() *k8s.KubeAuthenticationWebhookConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
				cfg.WebhookKubeconfig = "clusters: foo"

				return cfg
			},

			expectedError: "authentication webhook kubeconfig: kubeconfig is not valid: yaml: construct errors: line 1: cannot construct !!str `foo` into []k8s.webhookKubeconfigCluster",
		},
		{
			name: "kubeconfig with missing context",
			cfg: func() *k8s.KubeAuthenticationWebhookConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
				cfg.WebhookKubeconfig = `clusters:
  - name: webhook
    cluster:
      server: https://127.0.0.1/
current-context: webhook
`

				return cfg
			},

			expectedError: "authentication webhook kubeconfig: kubeconfig current-context \"webhook\" is not defined in contexts",
		},
		{
			name: "kubeconfig with missing cluster",
			cfg: func() *k8s.KubeAuthenticationWebhookConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
				cfg.WebhookKubeconfig = `contexts:
  - name: webhook
    context:
      cluster: webhook
current-context: webhook
`

				return cfg
			},

			expectedError: "authentication webhook kubeconfig: kubeconfig cluster \"webhook\" referenced by the current context is not defined in clusters",
		},
		{
			name: "kubeconfig with host files",
			cfg: func() *k8s.KubeAuthenticationWebhookConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
				cfg.WebhookKubeconfig = `clusters:
  - name: webhook
    cluster:
      certificate-authority: /var/lib/webhook/ca.crt
users:
  - name: apiserver
    user:
      client-certificate: /var/lib/webhook/client.crt
      client-key: /var/lib/webhook/client.key
      exec:
        command: get-token
contexts:
  - name: webhook
    context:
      cluster: webhook
      user: apiserver
current-context: webhook
`

				return cfg
			},

			expectedError: "authentication webhook kubeconfig: " +
				"kubeconfig cluster \"webhook\": \"certificate-authority\" is not supported, as kube-apiserver can't access host files, use \"certificate-authority-data\" instead\n" +
				"kubeconfig user \"apiserver\": \"client-certificate\" is not supported, as kube-apiserver can't access host files, use \"client-certificate-data\" instead\n" +
				"kubeconfig user \"apiserver\": \"client-key\" is not supported, as kube-apiserver can't access host files, use \"client-key-data\" instead\n" +
				"kubeconfig user \"apiserver\": \"exec\" is not supported, as kube-apiserver can't access host files\n" +
				"kubeconfig cluster \"webhook\" has no server",
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeAuthenticationWebhookConfigV1Alpha1 {
				cfg := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1()
				cfg.WebhookKubeconfig = testWebhookKubeconfig
				cfg.WebhookVersion = "v1beta1"
				cfg.WebhookCacheTTL = pointer.To(time.Minute)

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Empty(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubeAuthenticationWebhookConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with other extra args",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					APIServerConfig: &v1alpha1.APIServerConfig{ //nolint:staticcheck // testing deprecated field
						ExtraArgsConfig: meta.Args{
							"feature-gates": meta.NewArgValue("Foo=true", nil),
						},
					},
				},
			},
		},
		{
			name: "v1alpha1 with token webhook extra args",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					APIServerConfig: &v1alpha1.APIServerConfig{ //nolint:staticcheck // testing deprecated field
						ExtraArgsConfig: meta.Args{
							"authentication-token-webhook-config-file": meta.NewArgValue("/var/lib/webhook.kubeconfig", nil),
						},
					},
				},
			},

			expectedError: "kube-apiserver extra argument \"authentication-token-webhook-config-file\" is already set in v1alpha1 config (.cluster.apiServer.extraArgs)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeAuthenticationWebhookConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
