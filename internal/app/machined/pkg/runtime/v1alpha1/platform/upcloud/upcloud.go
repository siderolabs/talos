// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package upcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

const (
	// UpCloudMetadataEndpoint is the local UpCloud endpoint.
	UpCloudMetadataEndpoint = "http://169.254.169.254/metadata/v1.json"

	// UpCloudUserDataEndpoint is the local UpCloud endpoint for the config.
	UpCloudUserDataEndpoint = "http://169.254.169.254/metadata/v1/user_data"
)

// MetaData represents a metadata Upcloud interface.
type MetaData struct {
	Hostname   string   `json:"hostname,omitempty"`
	InstanceID string   `json:"instance_id,omitempty"`
	PublicKeys []string `json:"public_keys,omitempty"`
	Region     string   `json:"region,omitempty"`

	Network struct {
		Interfaces []struct {
			Index       int `json:"index,omitempty"`
			IPAddresses []struct {
				Address  string   `json:"address,omitempty"`
				DHCP     bool     `json:"dhcp,omitempty"`
				DNS      []string `json:"dns,omitempty"`
				Family   string   `json:"family,omitempty"`
				Floating bool     `json:"floating,omitempty"`
				Gateway  string   `json:"gateway,omitempty"`
				Network  string   `json:"network,omitempty"`
			} `json:"ip_addresses,omitempty"`
			MAC         string `json:"mac,omitempty"`
			NetworkType string `json:"type,omitempty"`
			NetworkID   string `json:"network_id,omitempty"`
		} `json:"interfaces,omitempty"`
		DNS []string `json:"dns,omitempty"`
	} `json:"network,omitempty"`
}

// UpCloud is the concrete type that implements the runtime.Platform interface.
type UpCloud struct{}

// Name implements the runtime.Platform interface.
func (u *UpCloud) Name() string {
	return "upcloud"
}

// ParseMetadata converts Upcloud metadata into platform network configuration.
//
//nolint:gocyclo
func (u *UpCloud) ParseMetadata(meta *MetaData) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if meta.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(meta.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	var dnsIPs []netaddr.IP

	firstIP := true

	for _, addr := range meta.Network.Interfaces {
		if addr.Index <= 0 { // protect from negative interface name
			continue
		}

		iface := fmt.Sprintf("eth%d", addr.Index-1)

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        iface,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		})

		for _, ip := range addr.IPAddresses {
			if firstIP {
				ipAddr, err := netaddr.ParseIP(ip.Address)
				if err != nil {
					return nil, err
				}

				networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ipAddr)

				firstIP = false
			}

			for _, addr := range ip.DNS {
				if ipAddr, err := netaddr.ParseIP(addr); err == nil {
					dnsIPs = append(dnsIPs, ipAddr)
				}
			}

			if ip.DHCP && ip.Family == "IPv4" {
				networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP4,
					LinkName:  iface,
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: 1024,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			}

			if !ip.DHCP {
				ntwrk, err := netaddr.ParseIPPrefix(ip.Network)
				if err != nil {
					return nil, err
				}

				addr, err := netaddr.ParseIP(ip.Address)
				if err != nil {
					return nil, err
				}

				ipPrefix := netaddr.IPPrefixFrom(addr, ntwrk.Bits())

				family := nethelpers.FamilyInet4
				if addr.Is6() {
					family = nethelpers.FamilyInet6
				}

				networkConfig.Addresses = append(networkConfig.Addresses,
					network.AddressSpecSpec{
						ConfigLayer: network.ConfigPlatform,
						LinkName:    iface,
						Address:     ipPrefix,
						Scope:       nethelpers.ScopeGlobal,
						Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
						Family:      family,
					},
				)

				if ip.Gateway != "" {
					gw, err := netaddr.ParseIP(ip.Gateway)
					if err != nil {
						return nil, err
					}

					route := network.RouteSpecSpec{
						ConfigLayer: network.ConfigPlatform,
						Gateway:     gw,
						Destination: ntwrk,
						OutLinkName: iface,
						Table:       nethelpers.TableMain,
						Protocol:    nethelpers.ProtocolStatic,
						Type:        nethelpers.TypeUnicast,
						Family:      family,
						Priority:    1024,
					}

					route.Normalize()

					networkConfig.Routes = append(networkConfig.Routes, route)
				}
			}
		}
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (u *UpCloud) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", UpCloudUserDataEndpoint)

	return download.Download(ctx, UpCloudUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (u *UpCloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (u *UpCloud) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (u *UpCloud) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching UpCloud instance config from: %q ", UpCloudMetadataEndpoint)

	metaConfigDl, err := download.Download(ctx, UpCloudMetadataEndpoint)
	if err != nil {
		return fmt.Errorf("failed to fetch network config from metadata service: %w", err)
	}

	meta := &MetaData{}
	if err = json.Unmarshal(metaConfigDl, meta); err != nil {
		return err
	}

	networkConfig, err := u.ParseMetadata(meta)
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
