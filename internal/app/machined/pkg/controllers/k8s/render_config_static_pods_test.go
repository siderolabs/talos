// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

const testWebhookKubeconfig = `apiVersion: v1
kind: Config
clusters:
  - name: webhook
    cluster:
      server: http://127.0.0.1:9051/
contexts:
  - name: webhook
    context:
      cluster: webhook
current-context: webhook
`

func TestAuthorizationConfigWebhookKubeconfig(t *testing.T) {
	t.Parallel()

	spec := &k8s.AuthorizationConfigSpec{
		Config: []k8s.AuthorizationAuthorizersSpec{
			{
				Type: "Node",
				Name: "node",
			},
			{
				Type: "Webhook",
				Name: "with-kubeconfig",
				Webhook: map[string]any{
					"timeout":       "3s",
					"failurePolicy": "Deny",
				},
				Kubeconfig: testWebhookKubeconfig,
			},
			{
				Type: "Webhook",
				Name: "in-cluster",
				Webhook: map[string]any{
					"connectionInfo": map[string]any{
						"type": "InClusterConfig",
					},
				},
			},
		},
	}

	obj, err := k8sctrl.AuthorizationConfig(spec, compatibility.VersionFromImageRef("registry.k8s.io/kube-apiserver:v1.34.0"))()
	require.NoError(t, err)

	cfg, ok := obj.(*apiserverv1.AuthorizationConfiguration)
	require.True(t, ok)
	require.Len(t, cfg.Authorizers, 3)

	assert.Nil(t, cfg.Authorizers[0].Webhook)

	require.NotNil(t, cfg.Authorizers[1].Webhook)
	assert.Equal(t, "Deny", cfg.Authorizers[1].Webhook.FailurePolicy)
	assert.Equal(t, apiserverv1.WebhookConnectionInfo{
		Type:           apiserverv1.AuthorizationWebhookConnectionInfoTypeKubeConfigFile,
		KubeConfigFile: new(filepath.Join(constants.KubernetesAPIServerConfigDir, "authorization-webhook-with-kubeconfig.kubeconfig")),
	}, cfg.Authorizers[1].Webhook.ConnectionInfo)

	require.NotNil(t, cfg.Authorizers[2].Webhook)
	assert.Equal(t, apiserverv1.WebhookConnectionInfo{
		Type: apiserverv1.AuthorizationWebhookConnectionInfoTypeInCluster,
	}, cfg.Authorizers[2].Webhook.ConnectionInfo)
}

func TestAPIServerWebhookKubeconfigs(t *testing.T) {
	t.Parallel()

	authorizationConfig := &k8s.AuthorizationConfigSpec{
		Config: []k8s.AuthorizationAuthorizersSpec{
			{
				Type: "RBAC",
				Name: "rbac",
			},
			{
				Type:       "Webhook",
				Name:       "authz",
				Webhook:    map[string]any{"timeout": "3s"},
				Kubeconfig: testWebhookKubeconfig,
			},
		},
	}

	kubeconfigs, err := k8sctrl.APIServerWebhookKubeconfigs(authorizationConfig, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"authorization-webhook-authz.kubeconfig": testWebhookKubeconfig,
	}, kubeconfigs)

	authenticationConfig := k8s.NewAuthenticationConfig()
	authenticationConfig.TypedSpec().TokenWebhook = &k8s.AuthenticationTokenWebhookSpec{
		Kubeconfig: testWebhookKubeconfig,
	}

	kubeconfigs, err = k8sctrl.APIServerWebhookKubeconfigs(authorizationConfig, authenticationConfig)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"authorization-webhook-authz.kubeconfig":  testWebhookKubeconfig,
		"authentication-token-webhook.kubeconfig": testWebhookKubeconfig,
	}, kubeconfigs)

	// a kubeconfig without a current context can't be used by kube-apiserver
	authenticationConfig.TypedSpec().TokenWebhook.Kubeconfig = "apiVersion: v1\nkind: Config\n"

	_, err = k8sctrl.APIServerWebhookKubeconfigs(authorizationConfig, authenticationConfig)
	assert.ErrorContains(t, err, "error validating authentication token webhook kubeconfig")
}

func TestRemoveStaleKubeconfigs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for _, name := range []string{
		"authorization-config.yaml",
		"authorization-webhook-current.kubeconfig",
		"authorization-webhook-stale.kubeconfig",
		"authentication-token-webhook.kubeconfig",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o400))
	}

	require.NoError(t, k8sctrl.RemoveStaleKubeconfigs(dir, map[string]string{
		"authorization-webhook-current.kubeconfig": "",
	}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	assert.Equal(t, []string{"authorization-config.yaml", "authorization-webhook-current.kubeconfig"}, names)
}
