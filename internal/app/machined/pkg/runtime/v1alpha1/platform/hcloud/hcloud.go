// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hcloud contains the Hcloud implementation of the [platform.Platform].
package hcloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Hcloud is the concrete type that implements the runtime.Platform interface.
type Hcloud struct{}

// Name implements the runtime.Platform interface.
func (h *Hcloud) Name() string {
	return "hcloud"
}

// ParseMetadata converts HCloud metadata to platform network configuration.
//
//nolint:gocyclo
func (h *Hcloud) ParseMetadata(unmarshalledNetworkConfig *NetworkConfig, metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	var publicIPs []string

	if metadata.PublicIPv4 != "" {
		publicIPs = append(publicIPs, metadata.PublicIPv4)
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
						RouteMetric: network.DefaultRouteMetric,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			}

			if subnet.Type == "static" {
				ipAddr, err := netip.ParsePrefix(subnet.Address)
				if err != nil {
					return nil, err
				}

				family := nethelpers.FamilyInet4

				if ipAddr.Addr().Is6() {
					publicIPs = append(publicIPs, strings.SplitN(subnet.Address, "/", 2)[0])
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
				gw, err := netip.ParseAddr(subnet.Gateway)
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
					Priority:    network.DefaultRouteMetric,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		}
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:   h.Name(),
		Hostname:   metadata.Hostname,
		Region:     metadata.Region,
		Zone:       metadata.AvailabilityZone,
		InstanceID: metadata.InstanceID,
		ProviderID: fmt.Sprintf("hcloud://%s", metadata.InstanceID),
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (h *Hcloud) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from: %q", HCloudUserDataEndpoint)

	configBytes, err := download.Download(ctx, HCloudUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, err
	}

	// Try to parse the downloaded config bytes as base64 string, so that users can provide the config in base64 format.
	// This also allows users to gzip this data, since the calling code will try to un-gzip the data if it detects it.
	return maybeBase64Decode(configBytes), nil
}

// maybeBase64Decode tries to interpret the provided bytes as base64 string and decode them.
// If the provided bytes are not a valid base64 string, the original bytes are returned.
func maybeBase64Decode(data []byte) []byte {
	out, err := base64.StdEncoding.AppendDecode(nil, data)
	if err != nil {
		return data
	}

	return out
}

// Mode implements the runtime.Platform interface.
func (h *Hcloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (h *Hcloud) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (h *Hcloud) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	metadata, err := h.getMetadata(ctx)
	if err != nil {
		return err
	}

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

	networkConfig, err := h.ParseMetadata(&unmarshalledNetworkConfig, metadata)
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
