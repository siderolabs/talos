// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/hashicorp/go-cleanhttp"

	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// NewResolver builds registry resolver based on Talos configuration.
func NewResolver(reg config.Registries) remotes.Resolver {
	return docker.NewResolver(docker.ResolverOptions{
		Hosts: RegistryHosts(reg),
	})
}

// RegistryHosts returns host configuration per registry.
//
//nolint:gocyclo
func RegistryHosts(reg config.Registries) docker.RegistryHosts {
	return func(host string) ([]docker.RegistryHost, error) {
		var registries []docker.RegistryHost

		endpoints, overridePath, err := RegistryEndpoints(reg, host)
		if err != nil {
			return nil, err
		}

		for _, endpoint := range endpoints {
			u, err := url.Parse(endpoint)
			if err != nil {
				return nil, fmt.Errorf("error parsing endpoint %q for host %q: %w", endpoint, host, err)
			}

			transport := newTransport()
			client := &http.Client{Transport: transport}

			registryConfig := reg.Config()[u.Host]

			if u.Scheme != "https" && registryConfig != nil && registryConfig.TLS() != nil {
				return nil, fmt.Errorf("TLS config specified for non-HTTPS registry: %q", u.Host)
			}

			if registryConfig != nil && registryConfig.TLS() != nil {
				transport.TLSClientConfig, err = registryConfig.TLS().GetTLSConfig()
				if err != nil {
					return nil, fmt.Errorf("error preparing TLS config for %q: %w", u.Host, err)
				}
			}

			if u.Path == "" {
				if !overridePath {
					u.Path = "/v2"
				}
			} else {
				u.Path = path.Clean(u.Path)

				if !strings.HasSuffix(u.Path, "/v2") && !overridePath {
					u.Path += "/v2"
				}
			}

			uu := u

			registries = append(registries, docker.RegistryHost{
				Client: client,
				Authorizer: docker.NewDockerAuthorizer(
					docker.WithAuthClient(client),
					docker.WithAuthCreds(func(host string) (string, string, error) {
						if registryConfig == nil {
							return "", "", nil
						}

						return PrepareAuth(registryConfig.Auth(), uu.Host, host)
					})),
				Host:         uu.Host,
				Scheme:       uu.Scheme,
				Path:         uu.Path,
				Capabilities: docker.HostCapabilityResolve | docker.HostCapabilityPull,
			})
		}

		return registries, nil
	}
}

// RegistryEndpoints returns registry endpoints per host using reg.
func RegistryEndpoints(reg config.Registries, host string) (endpoints []string, overridePath bool, err error) {
	// direct hit by host
	if hostConfig, ok := reg.Mirrors()[host]; ok {
		return hostConfig.Endpoints(), hostConfig.OverridePath(), nil
	}

	// '*'
	if catchAllConfig, ok := reg.Mirrors()["*"]; ok {
		return catchAllConfig.Endpoints(), catchAllConfig.OverridePath(), nil
	}

	// still no endpoints, use default
	defaultHost, err := docker.DefaultHost(host)
	if err != nil {
		return nil, false, fmt.Errorf("error getting default host for %q: %w", host, err)
	}

	return []string{"https://" + defaultHost}, false, nil
}

// PrepareAuth returns authentication info in the format expected by containerd.
func PrepareAuth(auth config.RegistryAuthConfig, host, expectedHost string) (string, string, error) {
	if auth == nil {
		return "", "", nil
	}

	if expectedHost != host {
		// don't pass auth to unknown hosts
		return "", "", nil
	}

	if auth.Username() != "" {
		return auth.Username(), auth.Password(), nil
	}

	if auth.IdentityToken() != "" {
		return "", auth.IdentityToken(), nil
	}

	if auth.Auth() != "" {
		decLen := base64.StdEncoding.DecodedLen(len(auth.Auth()))
		decoded := make([]byte, decLen)

		_, err := base64.StdEncoding.Decode(decoded, []byte(auth.Auth()))
		if err != nil {
			return "", "", fmt.Errorf("error parsing auth for %q: %w", host, err)
		}

		fields := strings.SplitN(string(decoded), ":", 2)
		if len(fields) != 2 {
			return "", "", fmt.Errorf("invalid decoded auth for %q: %q", host, decoded)
		}

		user, password := fields[0], fields[1]

		return user, strings.Trim(password, "\x00"), nil
	}

	return "", "", fmt.Errorf("invalid auth config for %q", host)
}

// newTransport creates HTTP transport with default settings.
func newTransport() *http.Transport {
	return httpdefaults.PatchTransport(cleanhttp.DefaultTransport())
}
