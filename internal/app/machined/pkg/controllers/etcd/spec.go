// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/net"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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
			ID:        optional.Some(etcd.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
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
//nolint:gocyclo,cyclop
func (ctrl *SpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

		nodeRoutedAddrs, err := safe.ReaderGet[*network.NodeAddress](
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

		nodeCurrentAddrs, err := safe.ReaderGet[*network.NodeAddress](
			ctx,
			r,
			resource.NewMetadata(
				network.NamespaceName,
				network.NodeAddressType,
				network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s),
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting addresses: %w", err)
		}

		routedAddrs := nodeRoutedAddrs.TypedSpec().IPs()
		currentAddrs := nodeCurrentAddrs.TypedSpec().IPs()

		// need at least a single address
		if len(routedAddrs) == 0 {
			continue
		}

		advertiseValidSubnets := etcdConfig.TypedSpec().AdvertiseValidSubnets

		if len(advertiseValidSubnets) == 0 {
			// not specified, advertise all addresses
			advertiseValidSubnets = []string{"0.0.0.0/0", "::/0"}
		}

		advertisedCIDRs := make([]string, 0, len(advertiseValidSubnets)+len(etcdConfig.TypedSpec().AdvertiseExcludeSubnets))
		advertisedCIDRs = append(advertisedCIDRs, advertiseValidSubnets...)
		advertisedCIDRs = append(advertisedCIDRs, xslices.Map(etcdConfig.TypedSpec().AdvertiseExcludeSubnets, func(cidr string) string { return "!" + cidr })...)

		listenCIDRs := make([]string, 0, len(etcdConfig.TypedSpec().ListenValidSubnets)+len(etcdConfig.TypedSpec().ListenExcludeSubnets))
		listenCIDRs = append(listenCIDRs, etcdConfig.TypedSpec().ListenValidSubnets...)
		listenCIDRs = append(listenCIDRs, xslices.Map(etcdConfig.TypedSpec().ListenExcludeSubnets, func(cidr string) string { return "!" + cidr })...)

		defaultListenAddress := netip.AddrFrom4([4]byte{0, 0, 0, 0})
		loopbackAddress := netip.AddrFrom4([4]byte{127, 0, 0, 1})

		for _, ip := range routedAddrs {
			if ip.Is6() {
				defaultListenAddress = netip.IPv6Unspecified()
				loopbackAddress = netip.MustParseAddr("::1")

				break
			}
		}

		var (
			advertisedIPs   []netip.Addr
			listenPeerIPs   []netip.Addr
			listenClientIPs []netip.Addr
		)

		if len(etcdConfig.TypedSpec().AdvertiseValidSubnets) == 0 {
			advertisedIPs, err = net.FilterIPs(routedAddrs, advertisedCIDRs)
			if err != nil {
				return fmt.Errorf("error filtering IPs: %w", err)
			}

			// if advertise subnet is not set, advertise the first address
			if len(advertisedIPs) > 0 {
				advertisedIPs = advertisedIPs[:1]
			}
		} else {
			advertisedIPs, err = net.FilterIPs(currentAddrs, advertisedCIDRs)
			if err != nil {
				return fmt.Errorf("error filtering IPs: %w", err)
			}
		}

		if len(listenCIDRs) > 0 {
			listenPeerIPs, err = net.FilterIPs(routedAddrs, listenCIDRs)
			if err != nil {
				return fmt.Errorf("error filtering IPs: %w", err)
			}

			listenClientIPs = append([]netip.Addr{loopbackAddress}, listenPeerIPs...)
		} else {
			listenPeerIPs = []netip.Addr{defaultListenAddress}
			listenClientIPs = []netip.Addr{defaultListenAddress}
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

		r.ResetRestartBackoff()
	}
}
