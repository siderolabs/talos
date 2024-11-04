// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/pair/ordered"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// LinkConfigController manages network.LinkSpec based on machine configuration, kernel cmdline.
type LinkConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *LinkConfigController) Name() string {
	return "network.LinkConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LinkConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.DeviceConfigSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LinkConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.LinkSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LinkConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		touchedIDs := make(map[resource.ID]struct{})

		items, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.DeviceConfigSpecType, "", resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		ignoredInterfaces := map[string]struct{}{}

		devices := make([]talosconfig.Device, len(items.Items))

		for i, item := range items.Items {
			device := item.(*network.DeviceConfigSpec).TypedSpec().Device
			devices[i] = device

			if device.Ignore() {
				ignoredInterfaces[device.Interface()] = struct{}{}
			}
		}

		// bring up loopback interface
		{
			var ids []string

			ids, err = ctrl.apply(ctx, r, []network.LinkSpecSpec{
				{
					Name:        "lo",
					Up:          true,
					ConfigLayer: network.ConfigDefault,
				},
			})
			if err != nil {
				return fmt.Errorf("error applying cmdline route: %w", err)
			}

			for _, id := range ids {
				touchedIDs[id] = struct{}{}
			}
		}

		// parse kernel cmdline for the interface name
		cmdlineLinks, cmdlineIgnored := ctrl.parseCmdline(logger)
		for _, cmdlineLink := range cmdlineLinks {
			if cmdlineLink.Name != "" {
				if _, ignored := ignoredInterfaces[cmdlineLink.Name]; !ignored {
					var ids []string

					ids, err = ctrl.apply(ctx, r, []network.LinkSpecSpec{cmdlineLink})
					if err != nil {
						return fmt.Errorf("error applying cmdline route: %w", err)
					}

					for _, id := range ids {
						touchedIDs[id] = struct{}{}
					}
				}
			}
		}

		// parse machine configuration for link specs
		if len(devices) > 0 {
			links := ctrl.processDevicesConfiguration(logger, devices)

			var ids []string

			ids, err = ctrl.apply(ctx, r, links)
			if err != nil {
				return fmt.Errorf("error applying machine configuration address: %w", err)
			}

			for _, id := range ids {
				touchedIDs[id] = struct{}{}
			}
		}

		// bring up any physical link not mentioned explicitly in the machine configuration
		configuredLinks := map[string]struct{}{}

		for _, linkName := range cmdlineIgnored {
			configuredLinks[linkName] = struct{}{}
		}

		for _, cmdlineLink := range cmdlineLinks {
			if cmdlineLink.Name != "" {
				configuredLinks[cmdlineLink.Name] = struct{}{}
			}
		}

		if len(devices) > 0 {
			for _, device := range devices {
				configuredLinks[device.Interface()] = struct{}{}

				if device.Bond() != nil {
					for _, link := range device.Bond().Interfaces() {
						configuredLinks[link] = struct{}{}
					}
				}

				if device.Bridge() != nil {
					for _, link := range device.Bridge().Interfaces() {
						configuredLinks[link] = struct{}{}
					}
				}
			}
		}

		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing link statuses: %w", err)
		}

		for _, item := range list.Items {
			linkStatus := item.(*network.LinkStatus) //nolint:errcheck,forcetypeassert

			if _, configured := configuredLinks[linkStatus.Metadata().ID()]; !configured {
				if linkStatus.TypedSpec().Physical() {
					var ids []string

					ids, err = ctrl.apply(ctx, r, []network.LinkSpecSpec{
						{
							Name:        linkStatus.Metadata().ID(),
							Up:          true,
							ConfigLayer: network.ConfigDefault,
						},
					})
					if err != nil {
						return fmt.Errorf("error applying default link up: %w", err)
					}

					for _, id := range ids {
						touchedIDs[id] = struct{}{}
					}
				}
			}
		}

		// list links for cleanup
		list, err = r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.LinkSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				// skip specs created by other controllers
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up routes: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *LinkConfigController) apply(ctx context.Context, r controller.Runtime, links []network.LinkSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(links))

	for _, link := range links {
		id := network.LayeredID(link.ConfigLayer, network.LinkID(link.Name))

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewLinkSpec(network.ConfigNamespaceName, id),
			func(r *network.LinkSpec) error {
				*r.TypedSpec() = link

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *LinkConfigController) parseCmdline(logger *zap.Logger) ([]network.LinkSpecSpec, []string) {
	if ctrl.Cmdline == nil {
		return []network.LinkSpecSpec{}, nil
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Info("ignoring error", zap.Error(err))

		return []network.LinkSpecSpec{}, nil
	}

	return settings.NetworkLinkSpecs, settings.IgnoreInterfaces
}

//nolint:gocyclo,cyclop
func (ctrl *LinkConfigController) processDevicesConfiguration(logger *zap.Logger, devices []talosconfig.Device) []network.LinkSpecSpec {
	// scan for the bonds or bridges
	bondedLinks := map[string]ordered.Pair[string, int]{} // mapping physical interface -> bond interface
	bridgedLinks := map[string]string{}                   // mapping physical interface -> bridge interface

	for _, device := range devices {
		if device.Ignore() {
			continue
		}

		if device.Bond() != nil {
			for idx, linkName := range device.Bond().Interfaces() {
				if bondData, exists := bondedLinks[linkName]; exists && bondData.F1 != device.Interface() {
					logger.Sugar().Warnf("link %q is included in both bonds %q and %q", linkName,
						bondData.F1, device.Interface())
				}

				if bridgeName, exists := bridgedLinks[linkName]; exists {
					logger.Sugar().Warnf("link %q is included in both bond %q and bridge %q", linkName,
						bridgeName, device.Interface())
				}

				bondedLinks[linkName] = ordered.MakePair(device.Interface(), idx)
			}
		}

		if device.Bridge() != nil {
			for _, linkName := range device.Bridge().Interfaces() {
				if bridgeName, exists := bridgedLinks[linkName]; exists && bridgeName != device.Interface() {
					logger.Sugar().Warnf("link %q is included in both bridges %q and %q", linkName,
						bridgeName, device.Interface())
				}

				if bondData, exists := bondedLinks[linkName]; exists {
					logger.Sugar().Warnf("link %q is included in both bond %q and bridge %q", linkName,
						bondData.F1, device.Interface())
				}

				bridgedLinks[linkName] = device.Interface()
			}
		}

		if device.BridgePort() != nil {
			if bridgeName, exists := bridgedLinks[device.Interface()]; exists && bridgeName != device.BridgePort().Master() {
				logger.Sugar().Warnf("link %q is included in both bridges %q and %q", device.Interface(),
					bridgeName, device.BridgePort().Master())
			}

			if bondData, exists := bondedLinks[device.Interface()]; exists {
				logger.Sugar().Warnf("link %q is included into both bond %q and bridge %q", device.Interface(),
					bondData.F1, device.BridgePort().Master())
			}

			bridgedLinks[device.Interface()] = device.BridgePort().Master()
		}
	}

	linkMap := map[string]*network.LinkSpecSpec{}

	for _, device := range devices {
		if device.Ignore() {
			continue
		}

		if _, exists := linkMap[device.Interface()]; !exists {
			linkMap[device.Interface()] = &network.LinkSpecSpec{
				Name:        device.Interface(),
				Up:          true,
				ConfigLayer: network.ConfigMachineConfiguration,
			}
		}

		if device.MTU() != 0 {
			linkMap[device.Interface()].MTU = uint32(device.MTU())
		}

		if device.Bond() != nil {
			if err := SetBondMaster(linkMap[device.Interface()], device.Bond()); err != nil {
				logger.Error("error parsing bond config", zap.Error(err))
			}
		}

		if device.Bridge() != nil {
			if err := SetBridgeMaster(linkMap[device.Interface()], device.Bridge()); err != nil {
				logger.Error("error parsing bridge config", zap.Error(err))
			}
		}

		if device.WireguardConfig() != nil {
			if err := wireguardLink(linkMap[device.Interface()], device.WireguardConfig()); err != nil {
				logger.Error("error parsing wireguard config", zap.Error(err))
			}
		}

		if device.Dummy() {
			dummyLink(linkMap[device.Interface()])
		}

		for _, vlan := range device.Vlans() {
			vlanName := nethelpers.VLANLinkName(device.Interface(), vlan.ID())

			linkMap[vlanName] = &network.LinkSpecSpec{
				Name:        device.Interface(),
				Up:          true,
				ConfigLayer: network.ConfigMachineConfiguration,
			}

			vlanLink(linkMap[vlanName], device.Interface(), vlan)
		}
	}

	for slaveName, bondData := range bondedLinks {
		if _, exists := linkMap[slaveName]; !exists {
			linkMap[slaveName] = &network.LinkSpecSpec{
				Name:        slaveName,
				Up:          true,
				ConfigLayer: network.ConfigMachineConfiguration,
			}
		}

		SetBondSlave(linkMap[slaveName], bondData)
	}

	for slaveName, bridgeIface := range bridgedLinks {
		if _, exists := linkMap[slaveName]; !exists {
			linkMap[slaveName] = &network.LinkSpecSpec{
				Name:        slaveName,
				Up:          true,
				ConfigLayer: network.ConfigMachineConfiguration,
			}
		}

		SetBridgeSlave(linkMap[slaveName], bridgeIface)
	}

	return maps.ValuesFunc(linkMap, func(link *network.LinkSpecSpec) network.LinkSpecSpec { return *link })
}

type vlaner interface {
	ID() uint16
	MTU() uint32
}

func vlanLink(link *network.LinkSpecSpec, linkName string, vlan vlaner) {
	link.Name = nethelpers.VLANLinkName(linkName, vlan.ID())
	link.Logical = true
	link.Up = true
	link.MTU = vlan.MTU()
	link.Kind = network.LinkKindVLAN
	link.Type = nethelpers.LinkEther
	link.ParentName = linkName
	link.VLAN = network.VLANSpec{
		VID:      vlan.ID(),
		Protocol: nethelpers.VLANProtocol8021Q,
	}
}

func wireguardLink(link *network.LinkSpecSpec, config talosconfig.WireguardConfig) error {
	link.Logical = true
	link.Kind = network.LinkKindWireguard
	link.Type = nethelpers.LinkNone
	link.Wireguard = network.WireguardSpec{
		PrivateKey:   config.PrivateKey(),
		ListenPort:   config.ListenPort(),
		FirewallMark: config.FirewallMark(),
	}

	for _, peer := range config.Peers() {
		allowedIPs := make([]netip.Prefix, 0, len(peer.AllowedIPs()))

		for _, allowedIP := range peer.AllowedIPs() {
			ip, err := netip.ParsePrefix(allowedIP)
			if err != nil {
				return err
			}

			allowedIPs = append(allowedIPs, ip)
		}

		link.Wireguard.Peers = append(link.Wireguard.Peers, network.WireguardPeer{
			PublicKey:                   peer.PublicKey(),
			Endpoint:                    peer.Endpoint(),
			PersistentKeepaliveInterval: peer.PersistentKeepaliveInterval(),
			AllowedIPs:                  allowedIPs,
		})
	}

	return nil
}

func dummyLink(link *network.LinkSpecSpec) {
	link.Logical = true
	link.Kind = "dummy"
	link.Type = nethelpers.LinkEther
}
