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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	cfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		// apply defaults for the loopback interface
		if err := ctrl.apply(ctx, r, ctrl.loopbackDefaults()); err != nil {
			return fmt.Errorf("error generating loopback interface defaults: %w", err)
		}

		devices, err := safe.ReaderListAll[*network.DeviceConfigSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error getting config: %w", err)
		}

		ignoredInterfaces := map[string]struct{}{}

		for device := range devices.All() {
			if device.TypedSpec().Device.Ignore() {
				ignoredInterfaces[device.TypedSpec().Device.Interface()] = struct{}{}
			}
		}

		// parse kernel cmdline for the address
		cmdlineAddresses := ctrl.parseCmdline(logger)
		for _, cmdlineAddress := range cmdlineAddresses {
			if _, ignored := ignoredInterfaces[cmdlineAddress.LinkName]; !ignored {
				if err = ctrl.apply(ctx, r, []network.AddressSpecSpec{cmdlineAddress}); err != nil {
					return fmt.Errorf("error applying cmdline address: %w", err)
				}
			}
		}

		// parse machine configuration for static addresses (legacy first)
		if devices.Len() > 0 {
			addresses := ctrl.processDevicesConfiguration(logger, devices)

			if err = ctrl.apply(ctx, r, addresses); err != nil {
				return fmt.Errorf("error applying machine configuration address: %w", err)
			}
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error reading machine config: %w", err)
			}
		}

		if cfg != nil {
			if err = ctrl.apply(ctx, r, ctrl.processMachineConfig(cfg.Config().NetworkCommonLinkConfigs())); err != nil {
				return fmt.Errorf("error applying machine configuration addresses: %w", err)
			}
		}

		if err := r.CleanupOutputs(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.AddressSpecType, "", resource.VersionUndefined)); err != nil {
			return fmt.Errorf("error during cleanup: %w", err)
		}
	}
}

//nolint:dupl
func (ctrl *AddressConfigController) apply(ctx context.Context, r controller.Runtime, addresses []network.AddressSpecSpec) error {
	for _, address := range addresses {
		id := network.LayeredID(address.ConfigLayer, network.AddressID(address.LinkName, address.Address))

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewAddressSpec(network.ConfigNamespaceName, id),
			func(r *network.AddressSpec) error {
				*r.TypedSpec() = address

				return nil
			},
		); err != nil {
			return err
		}
	}

	return nil
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
		return addresses
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline, network.NewEmptyLinkResolver())
	if err != nil {
		logger.Info("ignoring cmdline parse failure", zap.Error(err))

		return addresses
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

func (ctrl *AddressConfigController) processDevicesConfiguration(logger *zap.Logger, devices safe.List[*network.DeviceConfigSpec]) (addresses []network.AddressSpecSpec) {
	for item := range devices.All() {
		device := item.TypedSpec().Device

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

func (ctrl *AddressConfigController) processMachineConfig(linkConfigs []cfg.NetworkCommonLinkConfig) (addresses []network.AddressSpecSpec) {
	for _, linkConfig := range linkConfigs {
		for _, addr := range linkConfig.Addresses() {
			address := network.AddressSpecSpec{
				Address:     addr.Address(),
				Scope:       nethelpers.ScopeGlobal,
				LinkName:    linkConfig.Name(),
				ConfigLayer: network.ConfigMachineConfiguration,
				Priority:    addr.RoutePriority().ValueOrZero(),
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

	return addresses
}
