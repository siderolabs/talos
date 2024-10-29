// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// HostDNSConfigController manages network.HostDNSConfig based on machine configuration.
type HostDNSConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *HostDNSConfigController) Name() string {
	return "network.HostDNSConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *HostDNSConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *HostDNSConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.HostDNSConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: network.AddressSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *HostDNSConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var cfgProvider talosconfig.Config

		r.StartTrackingOutputs()

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else if cfg.Config().Machine() != nil {
			cfgProvider = cfg.Config()
		}

		newServiceAddrs := make([]netip.Addr, 0, 2)

		if err := safe.WriterModify(ctx, r, network.NewHostDNSConfig(network.HostDNSConfigID), func(res *network.HostDNSConfig) error {
			res.TypedSpec().ListenAddresses = []netip.AddrPort{
				netip.MustParseAddrPort("127.0.0.53:53"),
			}

			res.TypedSpec().ServiceHostDNSAddress = netip.Addr{}

			if cfgProvider == nil {
				res.TypedSpec().Enabled = false

				return nil
			}

			res.TypedSpec().Enabled = cfgProvider.Machine().Features().HostDNS().Enabled()
			res.TypedSpec().ResolveMemberNames = cfgProvider.Machine().Features().HostDNS().ResolveMemberNames()

			if !cfgProvider.Machine().Features().HostDNS().ForwardKubeDNSToHost() {
				return nil
			}

			if slices.ContainsFunc(
				cfgProvider.Cluster().Network().PodCIDRs(),
				func(cidr string) bool { return netip.MustParsePrefix(cidr).Addr().Is4() },
			) {
				parsed := netip.MustParseAddr(constants.HostDNSAddress)
				newServiceAddrs = append(newServiceAddrs, parsed)

				res.TypedSpec().ListenAddresses = append(res.TypedSpec().ListenAddresses, netip.AddrPortFrom(parsed, 53))
				res.TypedSpec().ServiceHostDNSAddress = parsed
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error writing host dns config: %w", err)
		}

		for _, newServiceAddr := range newServiceAddrs {
			err := updateSpec(ctx, r, newServiceAddr, logger)
			if err != nil {
				return err
			}
		}

		if err = safe.CleanupOutputs[*network.HostDNSConfig](ctx, r); err != nil {
			return err
		}
	}
}

func updateSpec(ctx context.Context, r controller.Runtime, newServiceAddr netip.Addr, logger *zap.Logger) error {
	newDNSAddrPrefix := netip.PrefixFrom(newServiceAddr, newServiceAddr.BitLen())

	logger.Debug("creating new host dns address spec", zap.String("address", newServiceAddr.String()))

	err := safe.WriterModify(
		ctx,
		r,
		network.NewAddressSpec(
			network.ConfigNamespaceName,
			network.LayeredID(network.ConfigOperator, network.AddressID("lo", newDNSAddrPrefix)),
		),
		func(r *network.AddressSpec) error {
			spec := r.TypedSpec()

			spec.Address = newDNSAddrPrefix
			spec.ConfigLayer = network.ConfigOperator

			if newServiceAddr.Is4() {
				spec.Family = nethelpers.FamilyInet4
			} else {
				spec.Family = nethelpers.FamilyInet6
			}

			spec.Flags = nethelpers.AddressFlags(nethelpers.AddressPermanent)
			spec.LinkName = "lo"

			if newServiceAddr.Is6() && newServiceAddr.IsPrivate() {
				spec.Scope = nethelpers.ScopeGlobal
			} else {
				spec.Scope = nethelpers.ScopeHost
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("error modifying address: %w", err)
	}

	return nil
}
