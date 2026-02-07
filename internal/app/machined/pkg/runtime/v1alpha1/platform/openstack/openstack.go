// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package openstack provides the OpenStack platform implementation.
package openstack

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/address"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// OpenStack is the concrete type that implements the runtime.Platform interface.
type OpenStack struct{}

// Name implements the runtime.Platform interface.
func (o *OpenStack) Name() string {
	return "openstack"
}

// ParseMetadata converts OpenStack metadata to platform network configuration.
//
//nolint:gocyclo,cyclop
func (o *OpenStack) ParseMetadata(
	ctx context.Context,
	unmarshalledNetworkConfig *NetworkConfig,
	extIPs []netip.Addr,
	metadata *MetadataConfig,
	st state.State,
) (*runtime.PlatformNetworkConfig, bool, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}
	needsReconcile := false

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, false, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	networkConfig.ExternalIPs = extIPs

	var dnsIPs []netip.Addr

	for _, netsvc := range unmarshalledNetworkConfig.Services {
		if netsvc.Type == "dns" && netsvc.Address != "" {
			if ip, err := netip.ParseAddr(netsvc.Address); err == nil {
				dnsIPs = append(dnsIPs, ip)
			} else {
				return nil, false, fmt.Errorf("failed to parse dns service ip: %w", err)
			}
		}
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	hostInterfaces, err := safe.StateListAll[*network.LinkStatus](ctx, st)
	if err != nil {
		return nil, false, fmt.Errorf("error listing host interfaces: %w", err)
	}

	ifaces := make(map[string]string)
	bondLinks := make(map[string]string)

	// Bonds

	bondIndex := 0

	for _, netLink := range unmarshalledNetworkConfig.Links {
		if netLink.Type != "bond" {
			continue
		}

		mode, err := nethelpers.BondModeByName(netLink.BondMode)
		if err != nil {
			return nil, false, fmt.Errorf("invalid bond_mode: %w", err)
		}

		hashPolicy, err := nethelpers.BondXmitHashPolicyByName(netLink.BondHashPolicy)
		if err != nil {
			return nil, false, fmt.Errorf("invalid bond_xmit_hash_policy: %w", err)
		}

		bondName := fmt.Sprintf("bond%d", bondIndex)
		ifaces[netLink.ID] = bondName

		bondLink := network.LinkSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Name:        bondName,
			Logical:     true,
			Up:          true,
			MTU:         uint32(netLink.MTU),
			Kind:        network.LinkKindBond,
			Type:        nethelpers.LinkEther,
			BondMaster: network.BondMasterSpec{
				Mode:       mode,
				MIIMon:     netLink.BondMIIMon,
				HashPolicy: hashPolicy,
			},
		}

		if netLink.Mac != "" {
			mac, err := net.ParseMAC(netLink.Mac)
			if err != nil {
				return nil, false, fmt.Errorf("invalid bond MAC address %q: %w", netLink.Mac, err)
			}

			bondLink.HardwareAddress = nethelpers.HardwareAddr(mac)
		}

		if mode == nethelpers.BondMode8023AD {
			bondLink.BondMaster.ADLACPActive = nethelpers.ADLACPActiveOn
		}

		networkadapter.BondMasterSpec(&bondLink.BondMaster).FillDefaults()
		networkConfig.Links = append(networkConfig.Links, bondLink)

		for _, link := range netLink.BondLinks {
			bondLinks[link] = bondName
		}

		bondIndex++
	}

	bondSlaveIndexes := make(map[string]int)

	// Interfaces

	for idx, netLink := range unmarshalledNetworkConfig.Links {
		// OpenStack network metadata schema:
		// "type": {
		// 	"$id": "#/definitions/l2_link/properties/type",
		// 	"type": "string",
		// 	"enum": [
		// 	  "bridge",
		// 	  "dvs",
		// 	  "hw_veb",
		// 	  "hyperv",
		// 	  "ovs",
		// 	  "tap",
		// 	  "vhostuser",
		// 	  "vif",
		// 	  "phy"
		// 	],
		// 	"title": "Interface type",
		// 	"examples": [
		// 	  "bridge"
		// 	]
		//   },
		//   "vif_id": {
		// 	"$ref": "#/definitions/l2_vif_id"
		//   }
		switch netLink.Type {
		case "phy", "vif", "ovs", "bridge", "tap", "vhostuser", "hw_veb":
			linkName := ""

			for hostInterface := range hostInterfaces.All() {
				macAddress := hostInterface.TypedSpec().PermanentAddr.String()
				if macAddress == "" {
					macAddress = hostInterface.TypedSpec().HardwareAddr.String()
				}

				if strings.EqualFold(macAddress, netLink.Mac) {
					linkName = hostInterface.Metadata().ID()

					break
				}
			}

			if linkName == "" {
				linkName = fmt.Sprintf("eth%d", idx)

				log.Printf("failed to find interface with MAC %q, using %q", netLink.Mac, linkName)

				needsReconcile = true
			}

			ifaces[netLink.ID] = linkName

			link := network.LinkSpecSpec{
				Name:        ifaces[netLink.ID],
				Up:          true,
				MTU:         uint32(netLink.MTU),
				ConfigLayer: network.ConfigPlatform,
			}

			if bondName, ok := bondLinks[netLink.ID]; ok {
				link.BondSlave = network.BondSlave{
					MasterName: bondName,
					SlaveIndex: bondSlaveIndexes[bondName],
				}

				bondSlaveIndexes[bondName]++
			}

			networkConfig.Links = append(networkConfig.Links, link)
		}
	}

	// VLANs
	for _, netLink := range unmarshalledNetworkConfig.Links {
		if netLink.Type != "vlan" {
			continue
		}

		parentName, ok := ifaces[netLink.VlanLink]
		if !ok {
			parentName = netLink.VlanLink
		}

		vlanName := fmt.Sprintf("%s.%d", parentName, netLink.VlanID)
		ifaces[netLink.ID] = vlanName

		vlanLink := network.LinkSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Name:        vlanName,
			Logical:     true,
			Up:          true,
			Kind:        network.LinkKindVLAN,
			Type:        nethelpers.LinkEther,
			ParentName:  parentName,
			VLAN: network.VLANSpec{
				VID:      netLink.VlanID,
				Protocol: nethelpers.VLANProtocol8021Q,
			},
		}

		if netLink.MTU != 0 {
			vlanLink.MTU = uint32(netLink.MTU)
		}

		networkConfig.Links = append(networkConfig.Links, vlanLink)
	}

	for _, ntwrk := range unmarshalledNetworkConfig.Networks {
		if ntwrk.ID == "" || ifaces[ntwrk.Link] == "" {
			continue
		}

		iface := ifaces[ntwrk.Link]

		switch ntwrk.Type {
		case "ipv4_dhcp":
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  iface,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric:         network.DefaultRouteMetric,
					SkipHostnameRequest: true,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		case "ipv6_dhcp", "ipv6_dhcpv6-stateless", "ipv6_dhcpv6-stateful":
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  iface,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric:         2 * network.DefaultRouteMetric,
					SkipHostnameRequest: true,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		case "ipv4", "ipv6", "ipv6_slaac":
			// FIXME: we need to switch on/off slaac here
		default:
			log.Printf("network type %s is not supported", ntwrk.Type)

			continue
		}

		if ntwrk.Address != "" {
			ipPrefix, err := address.IPPrefixFrom(ntwrk.Address, ntwrk.Netmask)
			if err != nil {
				return nil, false, fmt.Errorf("failed to parse ip address: %w", err)
			}

			family := nethelpers.FamilyInet4
			if ipPrefix.Addr().Is6() {
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

			if ntwrk.Gateway != "" {
				gw, err := netip.ParseAddr(ntwrk.Gateway)
				if err != nil {
					return nil, false, fmt.Errorf("failed to parse gateway ip: %w", err)
				}

				priority := uint32(network.DefaultRouteMetric)

				if family == nethelpers.FamilyInet6 {
					priority *= 2
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					OutLinkName: iface,
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      family,
					Priority:    priority,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		}

		for _, route := range ntwrk.Routes {
			gw, err := netip.ParseAddr(route.Gateway)
			if err != nil {
				return nil, false, fmt.Errorf("failed to parse route gateway: %w", err)
			}

			dest, err := address.IPPrefixFrom(route.Network, route.Netmask)
			if err != nil {
				return nil, false, fmt.Errorf("failed to parse route network: %w", err)
			}

			family := nethelpers.FamilyInet4
			if dest.Addr().Is6() {
				family = nethelpers.FamilyInet6
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Destination: dest,
				Gateway:     gw,
				OutLinkName: iface,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      family,
				Priority:    network.DefaultRouteMetric,
			}

			route.Normalize()

			// double the priority of the route if it is actually the default gateway and IPv6
			if route.Destination == (netip.Prefix{}) && family == nethelpers.FamilyInet6 {
				route.Priority *= 2
			}

			networkConfig.Routes = append(networkConfig.Routes, route)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     o.Name(),
		Hostname:     metadata.Hostname,
		Zone:         metadata.AvailabilityZone,
		InstanceID:   metadata.UUID,
		InstanceType: metadata.InstanceType,
		ProviderID:   fmt.Sprintf("openstack:///%s", metadata.UUID),
	}

	return networkConfig, needsReconcile, nil
}

// Configuration implements the runtime.Platform interface.
func (o *OpenStack) Configuration(ctx context.Context, r state.State) (machineConfig []byte, err error) {
	_, _, machineConfig, err = o.configFromCD(ctx, r)
	if err != nil {
		if err = netutils.Wait(ctx, r); err != nil {
			return nil, err
		}

		_, _, machineConfig, err = o.configFromNetwork(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Some openstack setups does not allow you to change user-data,
	// so skip this case.
	if bytes.HasPrefix(machineConfig, []byte("#cloud-config")) {
		return nil, errors.ErrNoConfigSource
	}

	return machineConfig, nil
}

// Mode implements the runtime.Platform interface.
func (o *OpenStack) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (o *OpenStack) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (o *OpenStack) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	// wait for devices to be ready before proceeding, otherwise we might not find network interfaces by MAC
	if err := netutils.WaitForDevicesReady(ctx, st); err != nil {
		return fmt.Errorf("error waiting for devices to be ready: %w", err)
	}

	networkSource := false

	metadataConfigDl, metadataNetworkConfigDl, _, err := o.configFromCD(ctx, st)
	if err != nil {
		metadataConfigDl, metadataNetworkConfigDl, _, err = o.configFromNetwork(ctx)
		if stderrors.Is(err, errors.ErrNoConfigSource) {
			err = nil
		}

		if err != nil {
			return err
		}

		networkSource = true
	}

	var (
		meta                      MetadataConfig
		unmarshalledNetworkConfig NetworkConfig
	)

	// ignore errors unmarshaling, empty configs work just fine as empty default
	_ = json.Unmarshal(metadataConfigDl, &meta)                             //nolint:errcheck
	_ = json.Unmarshal(metadataNetworkConfigDl, &unmarshalledNetworkConfig) //nolint:errcheck

	var extIPs []netip.Addr

	if networkSource {
		extIPs = o.externalIPs(ctx)

		if meta.InstanceType == "" {
			meta.InstanceType = o.instanceType(ctx)
		}
	}

	// do a loop to retry network config remap in case of missing links
	// on each try, export the configuration as it is, and if the network is reconciled next time, export the reconciled configuration
	bckoff := backoff.NewExponentialBackOff()

	for {
		networkConfig, needsReconcile, err := o.ParseMetadata(ctx, &unmarshalledNetworkConfig, extIPs, &meta, st)
		if err != nil {
			return err
		}

		select {
		case ch <- networkConfig:
		case <-ctx.Done():
			return ctx.Err()
		}

		if !needsReconcile {
			return nil
		}

		// wait for backoff to retry network config remap
		nextBackoff := bckoff.NextBackOff()
		if nextBackoff == backoff.Stop {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextBackoff):
		}
	}
}
