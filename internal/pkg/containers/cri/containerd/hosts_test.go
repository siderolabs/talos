// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/containers/cri/containerd"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

func TestGenerateHostsWithTLS(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*cri.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{EndpointEndpoint: "https://registry-1.docker.io"},
					{EndpointEndpoint: "https://registry-2.docker.io"},
				},
			},
		},
		auths: map[string]*cri.RegistryAuthConfig{
			"some.host:123": {
				RegistryUsername:      "root",
				RegistryPassword:      "secret",
				RegistryAuth:          "auth",
				RegistryIdentityToken: "token",
			},
		},
		tlses: map[string]*cri.RegistryTLSConfig{
			"some.host:123": {
				TLSInsecureSkipVerify: true,
				TLSCA:                 []byte("cacert"),
				TLSClientIdentity: &x509.PEMEncodedCertificateAndKey{
					Crt: []byte("clientcert"),
					Key: []byte("clientkey"),
				},
			},
			"registry-2.docker.io": {
				TLSInsecureSkipVerify: true,
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
						Contents: []byte("server = 'https://some.host:123'\nca = '/etc/cri/conf.d/hosts/some.host_123_/some.host:123-ca.crt'\nclient = [['/etc/cri/conf.d/hosts/some.host_123_/some.host:123-client.crt', '/etc/cri/conf.d/hosts/some.host_123_/some.host:123-client.key']]\nskip_verify = true\n"), //nolint:lll
					},
				},
			},
			"registry-2.docker.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("server = 'https://registry-2.docker.io'\nskip_verify = true\n"),
					},
				},
			},
		},
	}, result)
}

func TestGenerateHostsWithoutTLS(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*cri.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{EndpointEndpoint: "https://registry-1.docker.io"},
					{EndpointEndpoint: "https://registry-2.docker.io"},
				},
			},
			"*": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{EndpointEndpoint: "https://my-registry"},
				},
			},
		},
		auths: map[string]*cri.RegistryAuthConfig{
			"some.host:123": {
				RegistryUsername:      "root",
				RegistryPassword:      "secret",
				RegistryAuth:          "auth",
				RegistryIdentityToken: "token",
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
		mirrors: map[string]*cri.RegistryMirrorConfig{},
		tlses: map[string]*cri.RegistryTLSConfig{
			"*": {
				TLSCA: []byte("allcert"),
			},
		},
	}

	_, err := containerd.GenerateHosts(cfg, "/etc/cri/conf.d/hosts")
	assert.EqualError(t, err, "wildcard host TLS configuration is not supported")
}

func TestGenerateHostsTLSWildcard(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*cri.RegistryMirrorConfig{
			"*": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{EndpointEndpoint: "https://my-registry1"},
					{EndpointEndpoint: "https://my-registry2"},
				},
			},
		},
		tlses: map[string]*cri.RegistryTLSConfig{
			"my-registry1": {
				TLSCA: []byte("allcert"),
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
						Contents: []byte("server = 'https://my-registry1'\nca = '/etc/cri/conf.d/hosts/my-registry1/my-registry1-ca.crt'\n"),
					},
				},
			},
		},
	}, result)
}

func TestGenerateHostsWithHarbor(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*cri.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{
						EndpointEndpoint:     "https://harbor/v2/mirrors/proxy.docker.io",
						EndpointOverridePath: true,
					},
				},
			},
			"ghcr.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{
						EndpointEndpoint:     "https://harbor/v2/mirrors/proxy.ghcr.io",
						EndpointOverridePath: true,
					},
				},
			},
		},
		auths: map[string]*cri.RegistryAuthConfig{
			"harbor": {
				RegistryUsername:      "root",
				RegistryPassword:      "secret",
				RegistryAuth:          "auth",
				RegistryIdentityToken: "token",
			},
		},
		tlses: map[string]*cri.RegistryTLSConfig{
			"harbor": {
				TLSInsecureSkipVerify: true,
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
						Contents: []byte("server = 'https://harbor'\nskip_verify = true\n"),
					},
				},
			},
		},
	}, result)
}

func TestGenerateHostsSkipFallback(t *testing.T) {
	cfg := &mockConfig{
		mirrors: map[string]*cri.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{EndpointEndpoint: "https://harbor/v2/mirrors/proxy.docker.io", EndpointOverridePath: true},
					{EndpointEndpoint: "http://127.0.0.1:5001/v2/", EndpointOverridePath: true},
				},
				MirrorSkipFallback: true,
			},
			"ghcr.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{EndpointEndpoint: "http://127.0.0.1:5002"},
				},
				MirrorSkipFallback: true,
			},
		},
	}

	result, err := containerd.GenerateHosts(cfg, "/etc/cri/conf.d/hosts")
	require.NoError(t, err)

	t.Logf(
		"config docker.io %q",
		string(result.Directories["docker.io"].Files[0].Contents),
	)
	t.Logf(
		"config ghcr.io %q",
		string(result.Directories["ghcr.io"].Files[0].Contents),
	)

	assert.Equal(t, &containerd.HostsConfig{
		Directories: map[string]*containerd.HostsDirectory{
			"docker.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("server = 'http://127.0.0.1:5001/v2/'\ncapabilities = ['pull', 'resolve']\noverride_path = true\n[host]\n  [host.'https://harbor/v2/mirrors/proxy.docker.io']\n    capabilities = ['pull', 'resolve']\n    override_path = true\n"), //nolint:lll
					},
				},
			},
			"ghcr.io": {
				Files: []*containerd.HostsFile{
					{
						Name:     "hosts.toml",
						Mode:     0o600,
						Contents: []byte("server = 'http://127.0.0.1:5002'\ncapabilities = ['pull', 'resolve']\n"),
					},
				},
			},
		},
	}, result)
}
