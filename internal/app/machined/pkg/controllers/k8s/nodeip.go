// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/net"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// NodeIPController renders manifests based on templates and config/secrets.
type NodeIPController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeIPController) Name() string {
	return "k8s.NodeIPController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeIPController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeIPConfigType,
			ID:        pointer.ToString(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        pointer.ToString(network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s)),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeIPController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodeIPType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeIPController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodeIPConfigType, k8s.KubeletID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgSpec := cfg.(*k8s.NodeIPConfig).TypedSpec()

		cidrs := make([]string, 0, len(cfgSpec.ValidSubnets)+len(cfgSpec.ExcludeSubnets))

		cidrs = append(cidrs, cfgSpec.ValidSubnets...)

		for _, subnet := range cfgSpec.ExcludeSubnets {
			cidrs = append(cidrs, "!"+subnet)
		}

		// we have trigger on NodeAddresses, but we don't use them directly as they contain
		// some addresses which are not assigned to the node (like AWS ExternalIP).
		// we need to find solution for that later, for now just pull addresses directly

		ips, err := net.IPAddrs()
		if err != nil {
			return fmt.Errorf("error listing IPs: %w", err)
		}

		// we use stdnet.IP here to re-use already existing functions in talos-systems/net
		// once talos-systems/net is migrated to netaddr or netip, we can use it here
		ips = net.IPFilter(ips, network.NotSideroLinkStdIP)

		ips, err = net.FilterIPs(ips, cidrs)
		if err != nil {
			return fmt.Errorf("error filtering IPs: %w", err)
		}

		// filter down to make sure only one IPv4 and one IPv6 address stays
		var hasIPv4, hasIPv6 bool

		nodeIPs := make([]netaddr.IP, 0, 2)

		for _, ip := range ips {
			switch {
			case ip.To4() != nil:
				if !hasIPv4 {
					addr, _ := netaddr.FromStdIP(ip)
					nodeIPs = append(nodeIPs, addr)
					hasIPv4 = true
				} else {
					logger.Warn("node IP skipped, please use .machine.kubelet.nodeIP to provide explicit subnet for the node IP", zap.Stringer("address", ip))
				}
			case ip.To16() != nil:
				if !hasIPv6 {
					addr, _ := netaddr.FromStdIP(ip)
					nodeIPs = append(nodeIPs, addr)
					hasIPv6 = true
				} else {
					logger.Warn("node IP skipped, please use .machine.kubelet.nodeIP to provide explicit subnet for the node IP", zap.Stringer("address", ip))
				}
			}
		}

		if err = r.Modify(
			ctx,
			k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID),
			func(r resource.Resource) error {
				spec := r.(*k8s.NodeIP).TypedSpec()

				spec.Addresses = nodeIPs

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying NodeIP resource: %w", err)
		}
	}
}
