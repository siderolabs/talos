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
	"github.com/siderolabs/go-pointer"
	"github.com/talos-systems/net"
	"go.uber.org/zap"

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
			ID:        pointer.To(network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s)),
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

		cidrs := make([]string, 0, len(etcdConfig.TypedSpec().ValidSubnets)+len(etcdConfig.TypedSpec().ExcludeSubnets))

		cidrs = append(cidrs, etcdConfig.TypedSpec().ValidSubnets...)

		for _, subnet := range etcdConfig.TypedSpec().ExcludeSubnets {
			cidrs = append(cidrs, "!"+subnet)
		}

		// we have trigger on NodeAddresses, but we don't use them directly as they contain
		// some addresses which are not assigned to the node (like AWS ExternalIP).
		// we need to find solution for that later, for now just pull addresses directly

		ips, err := net.IPAddrs()
		if err != nil {
			return fmt.Errorf("error listing IPs: %w", err)
		}

		listenAddress := netip.IPv4Unspecified()

		for _, ip := range ips {
			if ip.To4() == nil {
				listenAddress = netip.IPv6Unspecified()

				break
			}
		}

		// we use stdnet.IP here to re-use already existing functions in talos-systems/net
		// once talos-systems/net is migrated to netaddr or netip, we can use it here
		ips = net.IPFilter(ips, network.NotSideroLinkStdIP)

		ips, err = net.FilterIPs(ips, cidrs)
		if err != nil {
			return fmt.Errorf("error filtering IPs: %w", err)
		}

		if len(ips) == 0 {
			continue
		}

		if err = safe.WriterModify(ctx, r, etcd.NewSpec(etcd.NamespaceName, etcd.SpecID), func(status *etcd.Spec) error {
			status.TypedSpec().AdvertisedAddress, _ = netip.AddrFromSlice(ips[0])
			status.TypedSpec().AdvertisedAddress = status.TypedSpec().AdvertisedAddress.Unmap()
			status.TypedSpec().ListenAddress = listenAddress
			status.TypedSpec().Name = hostnameStatus.TypedSpec().Hostname
			status.TypedSpec().Image = etcdConfig.TypedSpec().Image
			status.TypedSpec().ExtraArgs = etcdConfig.TypedSpec().ExtraArgs

			return nil
		}); err != nil {
			return fmt.Errorf("error updating Spec status: %w", err)
		}
	}
}
