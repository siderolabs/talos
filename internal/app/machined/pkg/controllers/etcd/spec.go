// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	stdnet "net"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"github.com/talos-systems/net"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/etcd"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// SpecController renders manifests based on templates and Spec/secrets.
type SpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *SpecController) Name() string {
	return "etcd.SpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.ConfigType,
			ID:        pointer.To(etcd.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        pointer.To(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        pointer.To(network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s)),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.SpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		etcdConfig, err := safe.ReaderGet[*etcd.Config](ctx, r, resource.NewMetadata(etcd.NamespaceName, etcd.ConfigType, etcd.ConfigID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting etcd config: %w", err)
		}

		hostnameStatus, err := safe.ReaderGet[*network.HostnameStatus](ctx, r, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting hostname status: %w", err)
		}

		nodeAddrs, err := safe.ReaderGet[*network.NodeAddress](
			ctx,
			r,
			resource.NewMetadata(
				network.NamespaceName,
				network.NodeAddressType,
				network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s),
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting addresses: %w", err)
		}

		addrs := nodeAddrs.TypedSpec().IPs()

		// need at least a single address
		if len(addrs) == 0 {
			continue
		}

		advertisedCIDRs := make([]string, 0, len(etcdConfig.TypedSpec().AdvertiseValidSubnets)+len(etcdConfig.TypedSpec().AdvertiseExcludeSubnets))
		advertisedCIDRs = append(advertisedCIDRs, etcdConfig.TypedSpec().AdvertiseValidSubnets...)
		advertisedCIDRs = append(advertisedCIDRs, slices.Map(etcdConfig.TypedSpec().AdvertiseExcludeSubnets, func(cidr string) string { return "!" + cidr })...)

		listenCIDRs := make([]string, 0, len(etcdConfig.TypedSpec().ListenValidSubnets)+len(etcdConfig.TypedSpec().ListenExcludeSubnets))
		listenCIDRs = append(listenCIDRs, etcdConfig.TypedSpec().ListenValidSubnets...)
		listenCIDRs = append(listenCIDRs, slices.Map(etcdConfig.TypedSpec().ListenExcludeSubnets, func(cidr string) string { return "!" + cidr })...)

		defaultListenAddress := netaddr.IPv4(0, 0, 0, 0)
		loopbackAddress := netaddr.IPv4(127, 0, 0, 1)

		for _, ip := range addrs {
			if ip.Is6() {
				defaultListenAddress = netaddr.IPv6Unspecified()
				loopbackAddress = netaddr.MustParseIP("::1")

				break
			}
		}

		var (
			advertisedIPs   []netaddr.IP
			listenPeerIPs   []netaddr.IP
			listenClientIPs []netaddr.IP
		)

		if len(advertisedCIDRs) > 0 {
			// TODO: this should eventually be rewritten with `net.FilterIPs` on netaddrs, but for now we'll keep same code and do the conversion.
			var stdIPs []stdnet.IP

			stdIPs, err = net.FilterIPs(nethelpers.MapNetAddrToStd(addrs), advertisedCIDRs)
			if err != nil {
				return fmt.Errorf("error filtering IPs: %w", err)
			}

			advertisedIPs = nethelpers.MapStdToNetAddr(stdIPs)
		} else {
			// if advertise subnet is not set, advertise the first address
			advertisedIPs = []netaddr.IP{addrs[0]}
		}

		if len(listenCIDRs) > 0 {
			// TODO: this should eventually be rewritten with `net.FilterIPs` on netaddrs, but for now we'll keep same code and do the conversion.
			var stdIPs []stdnet.IP

			stdIPs, err = net.FilterIPs(nethelpers.MapNetAddrToStd(addrs), listenCIDRs)
			if err != nil {
				return fmt.Errorf("error filtering IPs: %w", err)
			}

			listenPeerIPs = nethelpers.MapStdToNetAddr(stdIPs)
			listenClientIPs = append([]netaddr.IP{loopbackAddress}, listenPeerIPs...)
		} else {
			listenPeerIPs = []netaddr.IP{defaultListenAddress}
			listenClientIPs = []netaddr.IP{defaultListenAddress}
		}

		if len(advertisedIPs) == 0 || len(listenPeerIPs) == 0 {
			continue
		}

		if err = safe.WriterModify(ctx, r, etcd.NewSpec(etcd.NamespaceName, etcd.SpecID), func(status *etcd.Spec) error {
			status.TypedSpec().AdvertisedAddresses = advertisedIPs
			status.TypedSpec().ListenClientAddresses = listenClientIPs
			status.TypedSpec().ListenPeerAddresses = listenPeerIPs
			status.TypedSpec().Name = hostnameStatus.TypedSpec().Hostname
			status.TypedSpec().Image = etcdConfig.TypedSpec().Image
			status.TypedSpec().ExtraArgs = etcdConfig.TypedSpec().ExtraArgs

			return nil
		}); err != nil {
			return fmt.Errorf("error updating Spec status: %w", err)
		}
	}
}
