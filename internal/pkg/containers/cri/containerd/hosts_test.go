// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/containers/cri/containerd"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestGenerateHostsWithTLS(t *testing.T) {
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
					TLSInsecureSkipVerify: pointer.To(true),
					TLSCA:                 []byte("cacert"),
					TLSClientIdentity: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("clientcert"),
						Key: []byte("clientkey"),
					},
				},
			},
			"registry-2.docker.io": {
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSInsecureSkipVerify: pointer.To(true),
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
						Contents: []byte("[host]\n  [host.'https://registry-1.docker.io']\n    capabilities = ['pull', 'resolve']\n  [host.'https://registry-2.docker.io']\n    capabilities = ['pull', 'resolve']\n    skip_verify = true\n"), //nolint:lll
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
						Contents: []byte("[host]\n  [host.'https://some.host:123']\n    ca = '/etc/cri/conf.d/hosts/some.host_123_/some.host:123-ca.crt'\n    client = [['/etc/cri/conf.d/hosts/some.host_123_/some.host:123-client.crt', '/etc/cri/conf.d/hosts/some.host_123_/some.host:123-client.key']]\n    skip_verify = true\n"), //nolint:lll
					},
				},
			},
			"registry-2.docker.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("[host]\n  [host.'https://registry-2.docker.io']\n    skip_verify = true\n"),
					},
				},
			},
		},
	}, result)
}

func TestGenerateHostsWithoutTLS(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*v1alpha1.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []string{"https://registry-1.docker.io", "https://registry-2.docker.io"},
			},
			"*": {
				MirrorEndpoints: []string{"https://my-registry"},
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
						Contents: []byte("[host]\n  [host.'https://registry-1.docker.io']\n    capabilities = ['pull', 'resolve']\n  [host.'https://registry-2.docker.io']\n    capabilities = ['pull', 'resolve']\n"), //nolint:lll
					},
				},
			},
			"_default": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("[host]\n  [host.'https://my-registry']\n    capabilities = ['pull', 'resolve']\n"),
					},
				},
			},
		},
	}, result)
}

func TestGenerateHostsTLSWildcardWrong(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*v1alpha1.RegistryMirrorConfig{},
		config: map[string]*v1alpha1.RegistryConfig{
			"*": {
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSCA: []byte("allcert"),
				},
			},
		},
	}

	_, err := containerd.GenerateHosts(cfg, "/etc/cri/conf.d/hosts")
	assert.EqualError(t, err, "wildcard host TLS configuration is not supported")
}

func TestGenerateHostsTLSWildcard(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*v1alpha1.RegistryMirrorConfig{
			"*": {
				MirrorEndpoints: []string{"https://my-registry1", "https://my-registry2"},
			},
		},
		config: map[string]*v1alpha1.RegistryConfig{
			"my-registry1": {
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSCA: []byte("allcert"),
				},
			},
		},
	}

	result, err := containerd.GenerateHosts(cfg, "/etc/cri/conf.d/hosts")
	require.NoError(t, err)

	assert.Equal(t, &containerd.HostsConfig{
		Directories: map[string]*containerd.HostsDirectory{
			"_default": {
				Files: []*containerd.HostsFile{
					{
						Name:     "my-registry1-ca.crt",
						Mode:     0o600,
						Contents: []byte("allcert"),
					},
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("[host]\n  [host.'https://my-registry1']\n    capabilities = ['pull', 'resolve']\n    ca = '/etc/cri/conf.d/hosts/_default/my-registry1-ca.crt'\n  [host.'https://my-registry2']\n    capabilities = ['pull', 'resolve']\n"), //nolint:lll
					},
				},
			},
			"my-registry1": {
				Files: []*containerd.HostsFile{
					{
						Name:     "my-registry1-ca.crt",
						Mode:     0o600,
						Contents: []byte("allcert"),
					},
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("[host]\n  [host.'https://my-registry1']\n    ca = '/etc/cri/conf.d/hosts/my-registry1/my-registry1-ca.crt'\n"),
					},
				},
			},
		},
	}, result)
}

func TestGenerateHostsWithHarbor(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*v1alpha1.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints:    []string{"https://harbor/v2/mirrors/proxy.docker.io"},
				MirrorOverridePath: pointer.To(true),
			},
			"ghcr.io": {
				MirrorEndpoints:    []string{"https://harbor/v2/mirrors/proxy.ghcr.io"},
				MirrorOverridePath: pointer.To(true),
			},
		},
		config: map[string]*v1alpha1.RegistryConfig{
			"harbor": {
				RegistryAuth: &v1alpha1.RegistryAuthConfig{
					RegistryUsername:      "root",
					RegistryPassword:      "secret",
					RegistryAuth:          "auth",
					RegistryIdentityToken: "token",
				},
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSInsecureSkipVerify: pointer.To(true),
				},
			},
		},
	}

	result, err := containerd.GenerateHosts(cfg, "/etc/cri/conf.d/hosts")
	require.NoError(t, err)

	t.Logf(
		"config %q",
		string(result.Directories["harbor"].Files[0].Contents),
	)

	assert.Equal(t, &containerd.HostsConfig{
		Directories: map[string]*containerd.HostsDirectory{
			"docker.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("[host]\n  [host.'https://harbor/v2/mirrors/proxy.docker.io']\n    capabilities = ['pull', 'resolve']\n    override_path = true\n    skip_verify = true\n"),
					},
				},
			},
			"ghcr.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("[host]\n  [host.'https://harbor/v2/mirrors/proxy.ghcr.io']\n    capabilities = ['pull', 'resolve']\n    override_path = true\n    skip_verify = true\n"),
					},
				},
			},
			"harbor": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("[host]\n  [host.'https://harbor']\n    skip_verify = true\n"),
					},
				},
			},
		},
	}, result)
}
