// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/pelletier/go-toml"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

// HostsConfig describes layout of registry configuration in "hosts" format.
//
// See: https://github.com/containerd/containerd/blob/main/docs/hosts.md
type HostsConfig struct {
	Directories map[string]*HostsDirectory
}

// HostsDirectory describes a single directory for a specific registry.
type HostsDirectory struct {
	Files []*HostsFile
}

// HostsFile describes a single file configuring registry.
//
// This might be `hosts.toml` or a specific certificate.
type HostsFile struct {
	Name     string
	Contents []byte
	Mode     os.FileMode
}

// GenerateHosts generates a structure describing contents of the containerd hosts configuration.
//
//nolint:gocyclo
func GenerateHosts(cfg config.Registries, basePath string) (*HostsConfig, error) {
	config := &HostsConfig{
		Directories: map[string]*HostsDirectory{},
	}

	configureTLS := func(host string, directoryName string, hostToml *HostToml, directory *HostsDirectory) {
		tlsConfig, ok := cfg.Config()[host]
		if !ok {
			return
		}

		if tlsConfig.TLS().InsecureSkipVerify() {
			hostToml.SkipVerify = true
		}

		if tlsConfig.TLS().CA() != nil {
			relPath := fmt.Sprintf("%s-ca.crt", host)

			directory.Files = append(directory.Files,
				&HostsFile{
					Name:     relPath,
					Contents: tlsConfig.TLS().CA(),
					Mode:     0o600,
				},
			)

			hostToml.CACert = filepath.Join(basePath, directoryName, relPath)
		}

		if tlsConfig.TLS().ClientIdentity() != nil {
			relPathCrt := fmt.Sprintf("%s-client.crt", host)
			relPathKey := fmt.Sprintf("%s-client.key", host)

			directory.Files = append(directory.Files,
				&HostsFile{
					Name:     relPathCrt,
					Contents: tlsConfig.TLS().ClientIdentity().Crt,
					Mode:     0o600,
				},
				&HostsFile{
					Name:     relPathKey,
					Contents: tlsConfig.TLS().ClientIdentity().Key,
					Mode:     0o600,
				},
			)

			hostToml.Client = [][2]string{
				{
					filepath.Join(basePath, directoryName, relPathCrt),
					filepath.Join(basePath, directoryName, relPathKey),
				},
			}
		}
	}

	// process mirrors
	for registryName, endpoints := range cfg.Mirrors() {
		directoryName := hostDirectory(registryName)

		directory := &HostsDirectory{}

		// toml marshaling doesn't guarantee proper order of map keys, so instead we should marshal
		// each time and append to the output

		var buf bytes.Buffer

		enc := toml.NewEncoder(&buf)

		for _, endpoint := range endpoints.Endpoints() {
			hostsToml := HostsToml{
				HostConfigs: map[string]*HostToml{},
			}

			u, err := url.Parse(endpoint)
			if err != nil {
				return nil, fmt.Errorf("error parsing endpoint %q for host %q: %w", endpoint, registryName, err)
			}

			hostsToml.HostConfigs[endpoint] = &HostToml{
				Capabilities: []string{"pull", "resolve"}, // TODO: we should make it configurable eventually
			}

			configureTLS(u.Host, directoryName, hostsToml.HostConfigs[endpoint], directory)

			if err = enc.Encode(hostsToml); err != nil {
				return nil, err
			}
		}

		directory.Files = append(directory.Files,
			&HostsFile{
				Name:     "hosts.toml",
				Mode:     0o600,
				Contents: buf.Bytes(),
			},
		)

		config.Directories[directoryName] = directory
	}

	// process TLS config for non-mirrored endpoints (even if they were already processed)
	for hostname, tlsConfig := range cfg.Config() {
		directoryName := hostDirectory(hostname)

		if _, ok := config.Directories[directoryName]; ok {
			// skip, already configured
			continue
		}

		if tlsConfig.TLS().CA() == nil && tlsConfig.TLS().ClientIdentity() == nil && !tlsConfig.TLS().InsecureSkipVerify() {
			// skip, no specific config
			continue
		}

		directory := &HostsDirectory{}

		defaultHost, err := docker.DefaultHost(hostname)
		if err != nil {
			return nil, err
		}

		defaultHost = "https://" + defaultHost

		hostsToml := HostsToml{
			Server: defaultHost,
			HostConfigs: map[string]*HostToml{
				defaultHost: {},
			},
		}

		configureTLS(hostname, directoryName, hostsToml.HostConfigs[defaultHost], directory)

		marshaled, err := toml.Marshal(hostsToml)
		if err != nil {
			return nil, err
		}

		directory.Files = append(directory.Files,
			&HostsFile{
				Name:     "hosts.toml",
				Mode:     0o600,
				Contents: marshaled,
			},
		)

		config.Directories[directoryName] = directory
	}

	return config, nil
}

// hostDirectory converts ":port" to "_port_" in directory names.
func hostDirectory(host string) string {
	idx := strings.LastIndex(host, ":")
	if idx > 0 {
		return host[:idx] + "_" + host[idx+1:] + "_"
	}

	return host
}

// HostsToml describes the contents of the `hosts.toml` file.
type HostsToml struct {
	Server      string               `toml:"server,omitempty"`
	HostConfigs map[string]*HostToml `toml:"host"`
}

// HostToml is a single entry in `hosts.toml`.
type HostToml struct {
	Capabilities []string    `toml:"capabilities,omitempty"`
	CACert       string      `toml:"ca,omitempty"`
	Client       [][2]string `toml:"client,omitempty"`
	SkipVerify   bool        `toml:"skip_verify,omitempty"`
}
