// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vultr

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"

	"github.com/talos-systems/go-procfs/procfs"
	"github.com/vultr/metadata"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

const (
	// VultrMetadataEndpoint is the local Vultr endpoint fot the instance metadata.
	VultrMetadataEndpoint = "http://169.254.169.254/v1.json"
	// VultrExternalIPEndpoint is the local Vultr endpoint for the external IP.
	VultrExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"
	// VultrHostnameEndpoint is the local Vultr endpoint for the hostname.
	VultrHostnameEndpoint = "http://169.254.169.254/latest/meta-data/hostname"
	// VultrUserDataEndpoint is the local Vultr endpoint for the config.
	VultrUserDataEndpoint = "http://169.254.169.254/latest/user-data"
)

// Vultr is the concrete type that implements the runtime.Platform interface.
type Vultr struct{}

// Name implements the runtime.Platform interface.
func (v *Vultr) Name() string {
	return "vultr"
}

// ParseMetadata converts Vultr platform metadata into platform network config.
//
//nolint:gocyclo
func (v *Vultr) ParseMetadata(meta *metadata.MetaData, extIP []byte) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if ip, err := netaddr.ParseIP(string(extIP)); err == nil {
		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
	}

	if meta.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(meta.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	for i, addr := range meta.Interfaces {
		iface := fmt.Sprintf("eth%d", i)

		link := network.LinkSpecSpec{
			Name:        iface,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		}

		if addr.NetworkType == "private" {
			link.MTU = 1450
		}

		networkConfig.Links = append(networkConfig.Links, link)

		if addr.IPv4.Address != "" {
			if addr.NetworkType != "private" {
				networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP4,
					LinkName:  iface,
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: 1024,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			} else {
				maskIP, err := netaddr.ParseIP(addr.IPv4.Netmask)
				if err != nil {
					return nil, err
				}

				mask, _ := maskIP.MarshalBinary() //nolint:errcheck // never fails

				ip, err := netaddr.ParseIP(addr.IPv4.Address)
				if err != nil {
					return nil, err
				}

				ipAddr, err := ip.Netmask(mask)
				if err != nil {
					return nil, err
				}

				networkConfig.Addresses = append(networkConfig.Addresses,
					network.AddressSpecSpec{
						ConfigLayer: network.ConfigPlatform,
						LinkName:    iface,
						Address:     ipAddr,
						Scope:       nethelpers.ScopeGlobal,
						Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
						Family:      nethelpers.FamilyInet4,
					},
				)
			}
		}
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (v *Vultr) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", VultrUserDataEndpoint)

	return download.Download(ctx, VultrUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (v *Vultr) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (v *Vultr) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (v *Vultr) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching Vultr instance config from: %q ", VultrMetadataEndpoint)

	metaConfigDl, err := download.Download(ctx, VultrMetadataEndpoint)
	if err != nil {
		return fmt.Errorf("error fetching metadata: %w", err)
	}

	meta := &metadata.MetaData{}
	if err = json.Unmarshal(metaConfigDl, meta); err != nil {
		return err
	}

	extIP, err := download.Download(ctx, VultrExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil && !stderrors.Is(err, errors.ErrNoExternalIPs) {
		return err
	}

	networkConfig, err := v.ParseMetadata(meta, extIP)
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
