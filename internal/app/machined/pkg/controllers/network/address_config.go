// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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
			Namespace: network.NamespaceName,
			Type:      network.DeviceConfigSpecType,
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

		items, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.DeviceConfigSpecType, "", resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		ignoredInterfaces := map[string]struct{}{}

		devices := make([]config.Device, len(items.Items))

		for i, item := range items.Items {
			device := item.(*network.DeviceConfigSpec).TypedSpec().Device

			devices[i] = device

			if device.Ignore() {
				ignoredInterfaces[device.Interface()] = struct{}{}
			}
		}

		// parse kernel cmdline for the address
		cmdlineAddresses := ctrl.parseCmdline(logger)
		for _, cmdlineAddress := range cmdlineAddresses {
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
		if len(devices) > 0 {
			addresses := ctrl.processDevicesConfiguration(logger, devices)

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

		r.ResetRestartBackoff()
	}
}

//nolint:dupl
func (ctrl *AddressConfigController) apply(ctx context.Context, r controller.Runtime, addresses []network.AddressSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(addresses))

	for _, address := range addresses {
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
			Address:     netip.PrefixFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 8),
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeHost,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			LinkName:    "lo",
			ConfigLayer: network.ConfigDefault,
		},
	}
}

func (ctrl *AddressConfigController) parseCmdline(logger *zap.Logger) (addresses []network.AddressSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Info("ignoring cmdline parse failure", zap.Error(err))

		return
	}

	for _, linkConfig := range settings.LinkConfigs {
		if value.IsZero(linkConfig.Address) {
			continue
		}

		var address network.AddressSpecSpec

		address.Address = linkConfig.Address
		if address.Address.Addr().Is6() {
			address.Family = nethelpers.FamilyInet6
		} else {
			address.Family = nethelpers.FamilyInet4
		}

		address.Scope = nethelpers.ScopeGlobal
		address.Flags = nethelpers.AddressFlags(nethelpers.AddressPermanent)
		address.ConfigLayer = network.ConfigCmdline
		address.LinkName = linkConfig.LinkName

		addresses = append(addresses, address)
	}

	return addresses
}

func parseIPOrIPPrefix(address string) (netip.Prefix, error) {
	if strings.IndexByte(address, '/') >= 0 {
		return netip.ParsePrefix(address)
	}

	// parse as IP address and assume netmask of all ones
	ip, err := netip.ParseAddr(address)
	if err != nil {
		return netip.Prefix{}, err
	}

	return netip.PrefixFrom(ip, ip.BitLen()), nil
}

func (ctrl *AddressConfigController) processDevicesConfiguration(logger *zap.Logger, devices []config.Device) (addresses []network.AddressSpecSpec) {
	for _, device := range devices {
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

			if address.Address.Addr().Is6() {
				address.Family = nethelpers.FamilyInet6
			} else {
				address.Family = nethelpers.FamilyInet4
			}

			addresses = append(addresses, address)
		}

		for _, vlan := range device.Vlans() {
			for _, cidr := range vlan.Addresses() {
				ipPrefix, err := netip.ParsePrefix(cidr)
				if err != nil {
					logger.Info(fmt.Sprintf("skipping address %q on interface %q vlan %d", cidr, device.Interface(), vlan.ID()), zap.Error(err))

					continue
				}

				address := network.AddressSpecSpec{
					Address:     ipPrefix,
					Scope:       nethelpers.ScopeGlobal,
					LinkName:    nethelpers.VLANLinkName(device.Interface(), vlan.ID()),
					ConfigLayer: network.ConfigMachineConfiguration,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				}

				if address.Address.Addr().Is6() {
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
