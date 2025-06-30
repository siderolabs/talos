// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/pelletier/go-toml/v2"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
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

	configureEndpoint := func(host string, directoryName string, hostToml *HostToml, directory *HostsDirectory) {
		endpointConfig, ok := cfg.Config()[host]
		if !ok {
			return
		}

		if endpointConfig.TLS() != nil {
			if endpointConfig.TLS().InsecureSkipVerify() {
				hostToml.SkipVerify = true
			}

			if endpointConfig.TLS().CA() != nil {
				relPath := fmt.Sprintf("%s-ca.crt", host)

				directory.Files = append(directory.Files,
					&HostsFile{
						Name:     relPath,
						Contents: endpointConfig.TLS().CA(),
						Mode:     0o600,
					},
				)

				hostToml.CACert = filepath.Join(basePath, directoryName, relPath)
			}

			if endpointConfig.TLS().ClientIdentity() != nil {
				relPathCrt := fmt.Sprintf("%s-client.crt", host)
				relPathKey := fmt.Sprintf("%s-client.key", host)

				directory.Files = append(directory.Files,
					&HostsFile{
						Name:     relPathCrt,
						Contents: endpointConfig.TLS().ClientIdentity().Crt,
						Mode:     0o600,
					},
					&HostsFile{
						Name:     relPathKey,
						Contents: endpointConfig.TLS().ClientIdentity().Key,
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
	}

	// process mirrors
	for registryName, endpoints := range cfg.Mirrors() {
		directoryName := hostDirectory(registryName)

		directory := &HostsDirectory{}

		var hostsConfig HostsConfiguration

		for _, endpoint := range endpoints.Endpoints() {
			u, err := url.Parse(endpoint.Endpoint())
			if err != nil {
				return nil, fmt.Errorf("error parsing endpoint %q for host %q: %w", endpoint, registryName, err)
			}

			hostEntry := HostEntry{
				Host: endpoint.Endpoint(),
				HostToml: HostToml{
					Capabilities: []string{"pull", "resolve"}, // TODO: we should make it configurable eventually
					OverridePath: endpoint.OverridePath(),
				},
			}

			configureEndpoint(u.Host, directoryName, &hostEntry.HostToml, directory)

			hostsConfig.HostEntries = append(hostsConfig.HostEntries, hostEntry)
		}

		if endpoints.SkipFallback() {
			hostsConfig.DisableFallback()
		}

		cfgOut, err := hostsConfig.RenderTOML()
		if err != nil {
			return nil, err
		}

		directory.Files = append(directory.Files,
			&HostsFile{
				Name:     "hosts.toml",
				Mode:     0o600,
				Contents: cfgOut,
			},
		)

		config.Directories[directoryName] = directory
	}

	// process TLS config for non-mirrored endpoints (even if they were already processed)
	for hostname, registryConfig := range cfg.Config() {
		directoryName := hostDirectory(hostname)

		if _, ok := config.Directories[directoryName]; ok {
			// skip, already configured
			continue
		}

		if registryConfig.TLS() == nil || (registryConfig.TLS().CA() == nil && registryConfig.TLS().ClientIdentity() == nil && !registryConfig.TLS().InsecureSkipVerify()) {
			// skip, no specific config
			continue
		}

		if hostname == "*" {
			// no way to generate TLS config for wildcard host
			return nil, errors.New("wildcard host TLS configuration is not supported")
		}

		directory := &HostsDirectory{}

		defaultHost, err := docker.DefaultHost(hostname)
		if err != nil {
			return nil, err
		}

		defaultHost = "https://" + defaultHost

		rootEntry := HostEntry{
			Host: defaultHost,
		}

		configureEndpoint(hostname, directoryName, &rootEntry.HostToml, directory)

		hostsToml := HostsConfiguration{
			RootEntry: optional.Some(rootEntry),
		}

		cfgOut, err := hostsToml.RenderTOML()
		if err != nil {
			return nil, err
		}

		directory.Files = append(directory.Files,
			&HostsFile{
				Name:     "hosts.toml",
				Mode:     0o600,
				Contents: cfgOut,
			},
		)

		config.Directories[directoryName] = directory
	}

	return config, nil
}

// hostDirectory converts ":port" to "_port_" in directory names.
func hostDirectory(host string) string {
	if host == "*" {
		return "_default"
	}

	idx := strings.LastIndex(host, ":")
	if idx > 0 {
		return host[:idx] + "_" + host[idx+1:] + "_"
	}

	return host
}

// HostEntry describes the configuration for a single host.
type HostEntry struct {
	Host     string
	HostToml //nolint:embeddedstructfieldcheck
}

// HostsConfiguration describes the configuration of `hosts.toml` file in the format not compatible with TOML.
//
// The hosts entries should come in order, and go-toml only supports map[string]any, so we need to do some tricks.
type HostsConfiguration struct {
	RootEntry optional.Optional[HostEntry] // might be missing

	HostEntries []HostEntry
}

// DisableFallback disables the fallback to the default host.
func (hc *HostsConfiguration) DisableFallback() {
	if len(hc.HostEntries) == 0 {
		return
	}

	// push the last entry as the root entry
	hc.RootEntry = optional.Some(hc.HostEntries[len(hc.HostEntries)-1])

	hc.HostEntries = hc.HostEntries[:len(hc.HostEntries)-1]
}

// RenderTOML renders the configuration to TOML format.
func (hc *HostsConfiguration) RenderTOML() ([]byte, error) {
	var out bytes.Buffer

	// toml marshaling doesn't guarantee proper order of map keys, so instead we should marshal
	// each time and append to the output

	if rootEntry, ok := hc.RootEntry.Get(); ok {
		server := HostsTomlServer{
			Server:   rootEntry.Host,
			HostToml: rootEntry.HostToml,
		}

		if err := toml.NewEncoder(&out).SetIndentTables(true).Encode(server); err != nil {
			return nil, err
		}
	}

	for i, entry := range hc.HostEntries {
		hostEntry := HostsTomlHost{
			HostConfigs: map[string]HostToml{
				entry.Host: entry.HostToml,
			},
		}

		var tomlBuf bytes.Buffer

		if err := toml.NewEncoder(&tomlBuf).SetIndentTables(true).Encode(hostEntry); err != nil {
			return nil, err
		}

		tomlBytes := tomlBuf.Bytes()

		// this is an ugly hack, and neither TOML format nor go-toml library make it easier
		//
		// we need to marshal each endpoint in the order they are specified in the config, but go-toml defines
		// the tree as map[string]interface{} and doesn't guarantee the order of keys
		//
		// so we marshal each entry separately and combine the output, which results in something like:
		//
		//   [host]
		//     [host."foo.bar"]
		//	 [host]
		//     [host."bar.foo"]
		//
		// but this is invalid TOML, as `[host]' is repeated, so we do an ugly hack and remove it below
		const hostPrefix = "[host]\n"

		if i > 0 {
			if bytes.HasPrefix(tomlBytes, []byte(hostPrefix)) {
				tomlBytes = tomlBytes[len(hostPrefix):]
			}
		}

		out.Write(tomlBytes)
	}

	return out.Bytes(), nil
}

// HostsTomlServer describes only 'server' part of the `hosts.toml` file.
type HostsTomlServer struct {
	// top-level entry is used as the last one in the fallback chain.
	Server   string `toml:"server,omitempty"`
	HostToml        //nolint:embeddedstructfieldcheck       // embedded, matches the server
}

// HostsTomlHost describes the `hosts.toml` file entry for hosts.
//
// It is supposed to be marshaled as a single-entry map to keep the order correct.
type HostsTomlHost struct {
	// Note: this doesn't match the TOML format, but allows use to keep endpoints ordered properly.
	HostConfigs map[string]HostToml `toml:"host"`
}

// HostToml is a single entry in `hosts.toml`.
type HostToml struct {
	Capabilities []string    `toml:"capabilities,omitempty"`
	OverridePath bool        `toml:"override_path,omitempty"`
	CACert       string      `toml:"ca,omitempty"`
	Client       [][2]string `toml:"client,omitempty"`
	SkipVerify   bool        `toml:"skip_verify,omitempty"`
}
