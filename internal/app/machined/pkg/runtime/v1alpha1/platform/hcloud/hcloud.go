// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"

	"github.com/talos-systems/go-procfs/procfs"
	"gopkg.in/yaml.v3"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

const (
	// HCloudExternalIPEndpoint is the local hcloud endpoint for the external IP.
	HCloudExternalIPEndpoint = "http://169.254.169.254/hetzner/v1/metadata/public-ipv4"

	// HCloudNetworkEndpoint is the local hcloud endpoint for the network-config.
	HCloudNetworkEndpoint = "http://169.254.169.254/hetzner/v1/metadata/network-config"

	// HCloudHostnameEndpoint is the local hcloud endpoint for the hostname.
	HCloudHostnameEndpoint = "http://169.254.169.254/hetzner/v1/metadata/hostname"

	// HCloudUserDataEndpoint is the local hcloud endpoint for the config.
	HCloudUserDataEndpoint = "http://169.254.169.254/hetzner/v1/userdata"
)

// NetworkConfig holds hcloud network-config info.
type NetworkConfig struct {
	Version int `yaml:"version"`
	Config  []struct {
		Mac        string `yaml:"mac_address"`
		Interfaces string `yaml:"name"`
		Subnets    []struct {
			NameServers []string `yaml:"dns_nameservers,omitempty"`
			Address     string   `yaml:"address,omitempty"`
			Gateway     string   `yaml:"gateway,omitempty"`
			Ipv4        bool     `yaml:"ipv4,omitempty"`
			Ipv6        bool     `yaml:"ipv6,omitempty"`
			Type        string   `yaml:"type"`
		} `yaml:"subnets"`
		Type string `yaml:"type"`
	} `yaml:"config"`
}

// Hcloud is the concrete type that implements the runtime.Platform interface.
type Hcloud struct{}

// Name implements the runtime.Platform interface.
func (h *Hcloud) Name() string {
	return "hcloud"
}

// ParseMetadata converts HCloud metadata to platform network configuration.
//
//nolint:gocyclo
func (h *Hcloud) ParseMetadata(unmarshalledNetworkConfig *NetworkConfig, host, extIP []byte) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if len(host) > 0 {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(string(host)); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	if len(extIP) > 0 {
		if ip, err := netaddr.ParseIP(string(extIP)); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	for _, ntwrk := range unmarshalledNetworkConfig.Config {
		if ntwrk.Type != "physical" {
			continue
		}

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        ntwrk.Interfaces,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		})

		for _, subnet := range ntwrk.Subnets {
			if subnet.Type == "dhcp" && subnet.Ipv4 {
				networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
					Operator: network.OperatorDHCP4,
					LinkName: ntwrk.Interfaces,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: 1024,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			}

			if subnet.Type == "static" {
				ipAddr, err := netaddr.ParseIPPrefix(subnet.Address)
				if err != nil {
					return nil, err
				}

				family := nethelpers.FamilyInet4
				if ipAddr.IP().Is6() {
					family = nethelpers.FamilyInet6
				}

				networkConfig.Addresses = append(networkConfig.Addresses,
					network.AddressSpecSpec{
						ConfigLayer: network.ConfigPlatform,
						LinkName:    ntwrk.Interfaces,
						Address:     ipAddr,
						Scope:       nethelpers.ScopeGlobal,
						Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
						Family:      family,
					},
				)
			}

			if subnet.Gateway != "" && subnet.Ipv6 {
				gw, err := netaddr.ParseIP(subnet.Gateway)
				if err != nil {
					return nil, err
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					OutLinkName: ntwrk.Interfaces,
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet6,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		}
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (h *Hcloud) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", HCloudUserDataEndpoint)

	return download.Download(ctx, HCloudUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (h *Hcloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (h *Hcloud) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
//
//nolint:gocyclo
func (h *Hcloud) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching hcloud network config from: %q", HCloudNetworkEndpoint)

	metadataNetworkConfig, err := download.Download(ctx, HCloudNetworkEndpoint)
	if err != nil {
		return fmt.Errorf("failed to fetch network config from metadata service: %w", err)
	}

	var unmarshalledNetworkConfig NetworkConfig

	if err = yaml.Unmarshal(metadataNetworkConfig, &unmarshalledNetworkConfig); err != nil {
		return err
	}

	if unmarshalledNetworkConfig.Version != 1 {
		return fmt.Errorf("network-config metadata version=%d is not supported", unmarshalledNetworkConfig.Version)
	}

	log.Printf("fetching hostname from: %q", HCloudHostnameEndpoint)

	host, err := download.Download(ctx, HCloudHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil && !stderrors.Is(err, errors.ErrNoHostname) {
		return err
	}

	log.Printf("fetching externalIP from: %q", HCloudExternalIPEndpoint)

	extIP, err := download.Download(ctx, HCloudExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil && !stderrors.Is(err, errors.ErrNoExternalIPs) {
		return err
	}

	networkConfig, err := h.ParseMetadata(&unmarshalledNetworkConfig, host, extIP)
	if err != nil {
		return err
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
