// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"
	"inet.af/netaddr"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/network"
	"github.com/talos-systems/talos/pkg/resources/network/nethelpers"
)

// AddressConfigController manages network.AddressSpec based on machine configuration, kernel cmdline and some built-in defaults.
type AddressConfigController struct {
	Cmdline *procfs.Cmdline
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
//nolint: gocyclo, cyclop
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
			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up addresses: %w", err)
				}
			}
		}
	}
}

func (ctrl *AddressConfigController) apply(ctx context.Context, r controller.Runtime, addresses []network.AddressSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(addresses))

	for _, address := range addresses {
		address := address
		id := network.LayeredID(address.Layer, network.AddressID(address.LinkName, address.Address))

		if err := r.Modify(
			ctx,
			network.NewAddressSpec(network.ConfigNamespaceName, id),
			func(r resource.Resource) error {
				*r.(*network.AddressSpec).Status() = address

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
	return []network.AddressSpecSpec{
		{
			Address: netaddr.IPPrefix{
				IP:   netaddr.IPv4(127, 0, 0, 1),
				Bits: 8,
			},
			Family:   nethelpers.FamilyInet4,
			Scope:    nethelpers.ScopeHost,
			Flags:    nethelpers.AddressFlags(nethelpers.AddressPermanent),
			LinkName: "lo",
			Layer:    network.ConfigDefault,
		},
		{
			Address: netaddr.IPPrefix{
				IP:   netaddr.IPFrom16([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}),
				Bits: 128,
			},
			Family:   nethelpers.FamilyInet6,
			Scope:    nethelpers.ScopeHost,
			Flags:    nethelpers.AddressFlags(nethelpers.AddressPermanent),
			LinkName: "lo",
			Layer:    network.ConfigDefault,
		},
	}
}

//nolint: gocyclo
func (ctrl *AddressConfigController) parseCmdline(logger *zap.Logger) (address network.AddressSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	cmdline := ctrl.Cmdline.Get("ip").First()
	if cmdline == nil {
		return
	}

	// https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt
	// ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>
	fields := strings.Split(*cmdline, ":")

	// If dhcp is specified, we'll handle it as a normal discovered
	// interface
	if len(fields) == 1 && fields[0] == "dhcp" {
		return
	}

	var err error

	address.Address.IP, err = netaddr.ParseIP(fields[0])
	if err != nil {
		logger.Info("ignoring cmdline address parse failure", zap.Error(err))

		return
	}

	if len(fields) >= 4 {
		netmask, err := netaddr.ParseIP(fields[3])
		if err != nil {
			logger.Info("ignoring cmdline netmask parse failure", zap.Error(err))

			return
		}

		ones, _ := net.IPMask(netmask.IPAddr().IP).Size()

		address.Address.Bits = uint8(ones)
	} else {
		// default is to have complete address masked
		address.Address.Bits = address.Address.IP.BitLen()
	}

	if address.Address.IP.Is6() {
		address.Family = nethelpers.FamilyInet6
	} else {
		address.Family = nethelpers.FamilyInet4
	}

	address.Scope = nethelpers.ScopeGlobal
	address.Flags = nethelpers.AddressFlags(nethelpers.AddressPermanent)

	address.Layer = network.ConfigCmdline

	if len(fields) >= 6 {
		address.LinkName = fields[5]
	} else {
		ifaces, _ := net.Interfaces() //nolint: errcheck // ignoring error here as ifaces will be empty

		sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })

		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			address.LinkName = iface.Name

			break
		}
	}

	return address
}

func (ctrl *AddressConfigController) parseMachineConfiguration(logger *zap.Logger, cfgProvider talosconfig.Provider) (addresses []network.AddressSpecSpec) {
	for _, device := range cfgProvider.Machine().Network().Devices() {
		if device.Ignore() {
			continue
		}

		if device.CIDR() != "" {
			ipPrefix, err := netaddr.ParseIPPrefix(device.CIDR())
			if err != nil {
				logger.Info(fmt.Sprintf("skipping address %q on interface %q", device.CIDR(), device.Interface()), zap.Error(err))

				continue
			}

			address := network.AddressSpecSpec{
				Address:  ipPrefix,
				Scope:    nethelpers.ScopeGlobal,
				LinkName: device.Interface(),
				Layer:    network.ConfigMachineConfiguration,
				Flags:    nethelpers.AddressFlags(nethelpers.AddressPermanent),
			}

			if address.Address.IP.Is6() {
				address.Family = nethelpers.FamilyInet6
			} else {
				address.Family = nethelpers.FamilyInet4
			}

			addresses = append(addresses, address)
		}

		for _, vlan := range device.Vlans() {
			if vlan.CIDR() != "" {
				ipPrefix, err := netaddr.ParseIPPrefix(vlan.CIDR())
				if err != nil {
					logger.Info(fmt.Sprintf("skipping address %q on interface %q vlan %d", device.CIDR(), device.Interface(), vlan.ID()), zap.Error(err))

					continue
				}

				address := network.AddressSpecSpec{
					Address:  ipPrefix,
					Scope:    nethelpers.ScopeGlobal,
					LinkName: fmt.Sprintf("%s.%d", device.Interface(), vlan.ID()),
					Layer:    network.ConfigMachineConfiguration,
					Flags:    nethelpers.AddressFlags(nethelpers.AddressPermanent),
				}

				if address.Address.IP.Is6() {
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
