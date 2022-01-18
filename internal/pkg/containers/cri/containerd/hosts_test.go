// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

func TestGenerateHosts(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*v1alpha1.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []string{"https://registry-1.docker.io", "https://registry-2.docker.io"},
			},
		},
		config: map[string]*v1alpha1.RegistryConfig{
			"some.host:123": {
				RegistryAuth: &v1alpha1.RegistryAuthConfig{
					RegistryUsername:      "root",
					RegistryPassword:      "secret",
					RegistryAuth:          "auth",
					RegistryIdentityToken: "token",
				},
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSInsecureSkipVerify: true,
					TLSCA:                 []byte("cacert"),
					TLSClientIdentity: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("clientcert"),
						Key: []byte("clientkey"),
					},
				},
			},
			"registry-2.docker.io": {
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSInsecureSkipVerify: true,
				},
			},
		},
	}

	result, err := containerd.GenerateHosts(cfg, "/etc/cri/conf.d/hosts")
	require.NoError(t, err)

	assert.Equal(t, &containerd.HostsConfig{
		Directories: map[string]*containerd.HostsDirectory{
			"docker.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("\n[host]\n\n  [host.\"https://registry-1.docker.io\"]\n    capabilities = [\"pull\", \"resolve\"]\n\n[host]\n\n  [host.\"https://registry-2.docker.io\"]\n    capabilities = [\"pull\", \"resolve\"]\n    skip_verify = true\n"), //nolint:lll
					},
				},
			},
			"some.host_123_": {
				Files: []*containerd.HostsFile{
					{
						Name:     "some.host:123-ca.crt",
						Mode:     0o600,
						Contents: []byte("cacert"),
					},
					{
						Name:     "some.host:123-client.crt",
						Mode:     0o600,
						Contents: []byte("clientcert"),
					},
					{
						Name:     "some.host:123-client.key",
						Mode:     0o600,
						Contents: []byte("clientkey"),
					},
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("server = \"https://some.host:123\"\n\n[host]\n\n  [host.\"https://some.host:123\"]\n    ca = \"/etc/cri/conf.d/hosts/some.host_123_/some.host:123-ca.crt\"\n    client = [[\"/etc/cri/conf.d/hosts/some.host_123_/some.host:123-client.crt\", \"/etc/cri/conf.d/hosts/some.host_123_/some.host:123-client.key\"]]\n    skip_verify = true\n"), //nolint:lll
					},
				},
			},
			"registry-2.docker.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("server = \"https://registry-2.docker.io\"\n\n[host]\n\n  [host.\"https://registry-2.docker.io\"]\n    skip_verify = true\n"),
					},
				},
			},
		},
	}, result)
}
