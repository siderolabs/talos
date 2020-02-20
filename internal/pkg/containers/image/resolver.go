// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"golang.org/x/net/http/httpproxy"

	"github.com/talos-systems/talos/pkg/config/machine"
)

// NewResolver builds registry resolver based on Talos configuration.
func NewResolver(config machine.Registries) remotes.Resolver {
	return docker.NewResolver(docker.ResolverOptions{
		Hosts: RegistryHosts(config),
	})
}

// RegistryHosts returns host configuration per registry.
func RegistryHosts(config machine.Registries) docker.RegistryHosts {
	return func(host string) ([]docker.RegistryHost, error) {
		var registries []docker.RegistryHost

		endpoints, err := RegistryEndpoints(config, host)
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

			registryConfig := config.Config()[u.Host]

			if u.Scheme != "https" && registryConfig.TLS != nil {
				return nil, fmt.Errorf("TLS config specified for non-HTTPS registry: %q", u.Host)
			}

			if registryConfig.TLS != nil {
				transport.TLSClientConfig, err = registryConfig.TLS.GetTLSConfig()
				if err != nil {
					return nil, fmt.Errorf("error preparing TLS config for %q: %w", u.Host, err)
				}
			}

			if u.Path == "" {
				u.Path = "/v2"
			}

			uu := u

			registries = append(registries, docker.RegistryHost{
				Client: client,
				Authorizer: docker.NewDockerAuthorizer(
					docker.WithAuthClient(client),
					docker.WithAuthCreds(func(host string) (string, string, error) {
						return PrepareAuth(registryConfig.Auth, uu.Host, host)
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

// RegistryEndpoints returns registry endpoints per host using config.
func RegistryEndpoints(config machine.Registries, host string) ([]string, error) {
	var endpoints []string

	if hostConfig, ok := config.Mirrors()[host]; ok {
		endpoints = hostConfig.Endpoints
	}

	if endpoints == nil {
		if catchAllConfig, ok := config.Mirrors()["*"]; ok {
			endpoints = catchAllConfig.Endpoints
		}
	}

	if len(endpoints) == 0 {
		// still no endpoints, use default
		defaultHost, err := docker.DefaultHost(host)
		if err != nil {
			return nil, fmt.Errorf("error getting default host for %q: %w", host, err)
		}

		endpoints = append(endpoints, "https://"+defaultHost)
	}

	return endpoints, nil
}

// PrepareAuth returns authentication info in the format expected by containerd.
func PrepareAuth(auth *machine.RegistryAuthConfig, host, expectedHost string) (string, string, error) {
	if auth == nil {
		return "", "", nil
	}

	if expectedHost != host {
		// don't pass auth to unknown hosts
		return "", "", nil
	}

	if auth.Username != "" {
		return auth.Username, auth.Password, nil
	}

	if auth.IdentityToken != "" {
		return "", auth.IdentityToken, nil
	}

	if auth.Auth != "" {
		decLen := base64.StdEncoding.DecodedLen(len(auth.Auth))
		decoded := make([]byte, decLen)

		_, err := base64.StdEncoding.Decode(decoded, []byte(auth.Auth))
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
	return &http.Transport{
		// work around for  proxy.Do once bug.
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
	}
}
