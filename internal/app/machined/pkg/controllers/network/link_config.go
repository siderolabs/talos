// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"
	"inet.af/netaddr"

	networkadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/network"
	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
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

		var cfgProvider talosconfig.Provider

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			cfgProvider = cfg.(*config.MachineConfig).Config()
		}

		ignoredInterfaces := map[string]struct{}{}

		if cfgProvider != nil {
			for _, device := range cfgProvider.Machine().Network().Devices() {
				if device.Ignore() {
					ignoredInterfaces[device.Interface()] = struct{}{}
				}
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
		cmdlineLink, cmdlineIgnored := ctrl.parseCmdline(logger)
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

		// parse machine configuration for link specs
		if cfgProvider != nil {
			links := ctrl.parseMachineConfiguration(logger, cfgProvider)

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

		if cmdlineLink.Name != "" {
			configuredLinks[cmdlineLink.Name] = struct{}{}
		}

		if cfgProvider != nil {
			for _, device := range cfgProvider.Machine().Network().Devices() {
				configuredLinks[device.Interface()] = struct{}{}

				if device.Bond() != nil {
					for _, link := range device.Bond().Interfaces() {
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
				if linkStatus.Physical() {
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
	}
}

func (ctrl *LinkConfigController) apply(ctx context.Context, r controller.Runtime, links []network.LinkSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(links))

	for _, link := range links {
		link := link
		id := network.LayeredID(link.ConfigLayer, network.LinkID(link.Name))

		if err := r.Modify(
			ctx,
			network.NewLinkSpec(network.ConfigNamespaceName, id),
			func(r resource.Resource) error {
				*r.(*network.LinkSpec).TypedSpec() = link

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *LinkConfigController) parseCmdline(logger *zap.Logger) (network.LinkSpecSpec, []string) {
	if ctrl.Cmdline == nil {
		return network.LinkSpecSpec{}, nil
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Info("ignoring error", zap.Error(err))

		return network.LinkSpecSpec{}, nil
	}

	return network.LinkSpecSpec{
		Name:        settings.LinkName,
		Up:          true,
		ConfigLayer: network.ConfigCmdline,
	}, settings.IgnoreInterfaces
}

//nolint:gocyclo
func (ctrl *LinkConfigController) parseMachineConfiguration(logger *zap.Logger, cfgProvider talosconfig.Provider) []network.LinkSpecSpec {
	// scan for the bonds
	bondedLinks := map[string]string{} // mapping physical interface -> bond interface

	for _, device := range cfgProvider.Machine().Network().Devices() {
		if device.Ignore() {
			continue
		}

		if device.Bond() == nil {
			continue
		}

		for _, linkName := range device.Bond().Interfaces() {
			if bondName, exists := bondedLinks[linkName]; exists && bondName != device.Interface() {
				logger.Sugar().Warnf("link %q is included into more than two bonds", linkName)
			}

			bondedLinks[linkName] = device.Interface()
		}
	}

	linkMap := map[string]*network.LinkSpecSpec{}

	for _, device := range cfgProvider.Machine().Network().Devices() {
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
			if err := bondMaster(linkMap[device.Interface()], device.Bond()); err != nil {
				logger.Error("error parsing bond config", zap.Error(err))
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
			vlanSpec := vlanLink(device.Interface(), vlan)

			linkMap[vlanSpec.Name] = &vlanSpec
		}
	}

	for slaveName, bondName := range bondedLinks {
		if _, exists := linkMap[slaveName]; !exists {
			linkMap[slaveName] = &network.LinkSpecSpec{
				Name:        slaveName,
				Up:          true,
				ConfigLayer: network.ConfigMachineConfiguration,
			}
		}

		bondSlave(linkMap[slaveName], bondName)
	}

	links := make([]network.LinkSpecSpec, 0, len(linkMap))

	for _, link := range linkMap {
		links = append(links, *link)
	}

	return links
}

func bondSlave(link *network.LinkSpecSpec, bondName string) {
	link.MasterName = bondName
}

//nolint:gocyclo
func bondMaster(link *network.LinkSpecSpec, bond talosconfig.Bond) error {
	link.Logical = true
	link.Kind = network.LinkKindBond
	link.Type = nethelpers.LinkEther

	bondMode, err := nethelpers.BondModeByName(bond.Mode())
	if err != nil {
		return err
	}

	hashPolicy, err := nethelpers.BondXmitHashPolicyByName(bond.HashPolicy())
	if err != nil {
		return err
	}

	lacpRate, err := nethelpers.LACPRateByName(bond.LACPRate())
	if err != nil {
		return err
	}

	arpValidate, err := nethelpers.ARPValidateByName(bond.ARPValidate())
	if err != nil {
		return err
	}

	arpAllTargets, err := nethelpers.ARPAllTargetsByName(bond.ARPAllTargets())
	if err != nil {
		return err
	}

	var primary uint32

	if bond.Primary() != "" {
		var iface *net.Interface

		iface, err = net.InterfaceByName(bond.Primary())
		if err != nil {
			return err
		}

		primary = uint32(iface.Index)
	}

	primaryReselect, err := nethelpers.PrimaryReselectByName(bond.PrimaryReselect())
	if err != nil {
		return err
	}

	failOverMAC, err := nethelpers.FailOverMACByName(bond.FailOverMac())
	if err != nil {
		return err
	}

	adSelect, err := nethelpers.ADSelectByName(bond.ADSelect())
	if err != nil {
		return err
	}

	link.BondMaster = network.BondMasterSpec{
		Mode:            bondMode,
		HashPolicy:      hashPolicy,
		LACPRate:        lacpRate,
		ARPValidate:     arpValidate,
		ARPAllTargets:   arpAllTargets,
		PrimaryIndex:    primary,
		PrimaryReselect: primaryReselect,
		FailOverMac:     failOverMAC,
		ADSelect:        adSelect,
		MIIMon:          bond.MIIMon(),
		UpDelay:         bond.UpDelay(),
		DownDelay:       bond.DownDelay(),
		ARPInterval:     bond.ARPInterval(),
		ResendIGMP:      bond.ResendIGMP(),
		MinLinks:        bond.MinLinks(),
		LPInterval:      bond.LPInterval(),
		PacketsPerSlave: bond.PacketsPerSlave(),
		NumPeerNotif:    bond.NumPeerNotif(),
		TLBDynamicLB:    bond.TLBDynamicLB(),
		AllSlavesActive: bond.AllSlavesActive(),
		UseCarrier:      bond.UseCarrier(),
		ADActorSysPrio:  bond.ADActorSysPrio(),
		ADUserPortKey:   bond.ADUserPortKey(),
		PeerNotifyDelay: bond.PeerNotifyDelay(),
	}
	networkadapter.BondMasterSpec(&link.BondMaster).FillDefaults()

	return nil
}

func vlanLink(linkName string, vlan talosconfig.Vlan) network.LinkSpecSpec {
	return network.LinkSpecSpec{
		Name:       fmt.Sprintf("%s.%d", linkName, vlan.ID()),
		Logical:    true,
		Up:         true,
		MTU:        vlan.MTU(),
		Kind:       network.LinkKindVLAN,
		Type:       nethelpers.LinkEther,
		ParentName: linkName,
		VLAN: network.VLANSpec{
			VID:      vlan.ID(),
			Protocol: nethelpers.VLANProtocol8021Q,
		},
		ConfigLayer: network.ConfigMachineConfiguration,
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
		allowedIPs := make([]netaddr.IPPrefix, 0, len(peer.AllowedIPs()))

		for _, allowedIP := range peer.AllowedIPs() {
			ip, err := netaddr.ParseIPPrefix(allowedIP)
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
