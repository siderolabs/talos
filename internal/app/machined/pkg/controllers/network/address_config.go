// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// AddressConfigController manages network.AddressSpec based on machine configuration, kernel cmdline and some built-in defaults.
type AddressConfigController struct {
	Cmdline      *procfs.Cmdline
	V1Alpha1Mode runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *AddressConfigController) Name() string {
	return "network.AddressConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AddressConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *AddressConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.AddressSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *AddressConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// apply defaults for the loopback interface once
	defaultTouchedIDs, err := ctrl.apply(ctx, r, ctrl.loopbackDefaults())
	if err != nil {
		return fmt.Errorf("error generating loopback interface defaults: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		touchedIDs := make(map[resource.ID]struct{})

		for _, id := range defaultTouchedIDs {
			touchedIDs[id] = struct{}{}
		}

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

		// parse kernel cmdline for the address
		cmdlineAddress := ctrl.parseCmdline(logger)
		if !cmdlineAddress.Address.IsZero() {
			if _, ignored := ignoredInterfaces[cmdlineAddress.LinkName]; !ignored {
				var ids []string

				ids, err = ctrl.apply(ctx, r, []network.AddressSpecSpec{cmdlineAddress})
				if err != nil {
					return fmt.Errorf("error applying cmdline address: %w", err)
				}

				for _, id := range ids {
					touchedIDs[id] = struct{}{}
				}
			}
		}

		// parse machine configuration for static addresses
		if cfgProvider != nil {
			addresses := ctrl.parseMachineConfiguration(logger, cfgProvider)

			var ids []string

			ids, err = ctrl.apply(ctx, r, addresses)
			if err != nil {
				return fmt.Errorf("error applying machine configuration address: %w", err)
			}

			for _, id := range ids {
				touchedIDs[id] = struct{}{}
			}
		}

		// list addresses for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.AddressSpecType, "", resource.VersionUndefined))
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
					return fmt.Errorf("error cleaning up addresses: %w", err)
				}
			}
		}
	}
}

//nolint:dupl
func (ctrl *AddressConfigController) apply(ctx context.Context, r controller.Runtime, addresses []network.AddressSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(addresses))

	for _, address := range addresses {
		address := address
		id := network.LayeredID(address.ConfigLayer, network.AddressID(address.LinkName, address.Address))

		if err := r.Modify(
			ctx,
			network.NewAddressSpec(network.ConfigNamespaceName, id),
			func(r resource.Resource) error {
				*r.(*network.AddressSpec).TypedSpec() = address

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *AddressConfigController) loopbackDefaults() []network.AddressSpecSpec {
	if ctrl.V1Alpha1Mode == runtime.ModeContainer {
		// skip configuring lo addresses in container mode
		return nil
	}

	return []network.AddressSpecSpec{
		{
			Address:     netaddr.IPPrefixFrom(netaddr.IPv4(127, 0, 0, 1), 8),
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeHost,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			LinkName:    "lo",
			ConfigLayer: network.ConfigDefault,
		},
	}
}

func (ctrl *AddressConfigController) parseCmdline(logger *zap.Logger) (address network.AddressSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Info("ignoring cmdline parse failure", zap.Error(err))

		return
	}

	if settings.Address.IsZero() {
		return
	}

	address.Address = settings.Address
	if address.Address.IP().Is6() {
		address.Family = nethelpers.FamilyInet6
	} else {
		address.Family = nethelpers.FamilyInet4
	}

	address.Scope = nethelpers.ScopeGlobal
	address.Flags = nethelpers.AddressFlags(nethelpers.AddressPermanent)
	address.ConfigLayer = network.ConfigCmdline
	address.LinkName = settings.LinkName

	return address
}

func parseIPOrIPPrefix(address string) (netaddr.IPPrefix, error) {
	if strings.IndexByte(address, '/') >= 0 {
		return netaddr.ParseIPPrefix(address)
	}

	// parse as IP address and assume netmask of all ones
	ip, err := netaddr.ParseIP(address)
	if err != nil {
		return netaddr.IPPrefix{}, err
	}

	return netaddr.IPPrefixFrom(ip, ip.BitLen()), nil
}

func (ctrl *AddressConfigController) parseMachineConfiguration(logger *zap.Logger, cfgProvider talosconfig.Provider) (addresses []network.AddressSpecSpec) {
	for _, device := range cfgProvider.Machine().Network().Devices() {
		if device.Ignore() {
			continue
		}

		for _, cidr := range device.Addresses() {
			ipPrefix, err := parseIPOrIPPrefix(cidr)
			if err != nil {
				logger.Info(fmt.Sprintf("skipping address %q on interface %q", cidr, device.Interface()), zap.Error(err))

				continue
			}

			address := network.AddressSpecSpec{
				Address:     ipPrefix,
				Scope:       nethelpers.ScopeGlobal,
				LinkName:    device.Interface(),
				ConfigLayer: network.ConfigMachineConfiguration,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			}

			if address.Address.IP().Is6() {
				address.Family = nethelpers.FamilyInet6
			} else {
				address.Family = nethelpers.FamilyInet4
			}

			addresses = append(addresses, address)
		}

		for _, vlan := range device.Vlans() {
			for _, cidr := range vlan.Addresses() {
				ipPrefix, err := netaddr.ParseIPPrefix(cidr)
				if err != nil {
					logger.Info(fmt.Sprintf("skipping address %q on interface %q vlan %d", cidr, device.Interface(), vlan.ID()), zap.Error(err))

					continue
				}

				address := network.AddressSpecSpec{
					Address:     ipPrefix,
					Scope:       nethelpers.ScopeGlobal,
					LinkName:    fmt.Sprintf("%s.%d", device.Interface(), vlan.ID()),
					ConfigLayer: network.ConfigMachineConfiguration,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				}

				if address.Address.IP().Is6() {
					address.Family = nethelpers.FamilyInet6
				} else {
					address.Family = nethelpers.FamilyInet4
				}

				addresses = append(addresses, address)
			}
		}
	}

	return addresses
}
