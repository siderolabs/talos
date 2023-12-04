// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

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

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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
			ID:        optional.Some(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s)),
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

		cfg, err := safe.ReaderGet[*k8s.NodeIPConfig](ctx, r, resource.NewMetadata(k8s.NamespaceName, k8s.NodeIPConfigType, k8s.KubeletID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgSpec := cfg.TypedSpec()

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

		cidrs := make([]string, 0, len(cfgSpec.ValidSubnets)+len(cfgSpec.ExcludeSubnets))
		cidrs = append(cidrs, cfgSpec.ValidSubnets...)
		cidrs = append(cidrs, xslices.Map(cfgSpec.ExcludeSubnets, func(cidr string) string { return "!" + cidr })...)

		ips, err := net.FilterIPs(addrs, cidrs)
		if err != nil {
			return fmt.Errorf("error filtering IPs: %w", err)
		}

		if len(ips) == 0 {
			logger.Warn("no suitable node IP found, please make sure .machine.kubelet.nodeIP filters and pod/service subnets are set up correctly")

			continue
		}

		// filter down to make sure only one IPv4 and one IPv6 address stays
		var hasIPv4, hasIPv6 bool

		nodeIPs := make([]netip.Addr, 0, 2)

		for _, ip := range ips {
			switch {
			case ip.Is4():
				if !hasIPv4 {
					nodeIPs = append(nodeIPs, ip)
					hasIPv4 = true
				} else {
					logger.Warn("node IP skipped, please use .machine.kubelet.nodeIP to provide explicit subnet for the node IP", zap.Stringer("address", ip))
				}
			case ip.Is6():
				if !hasIPv6 {
					nodeIPs = append(nodeIPs, ip)
					hasIPv6 = true
				} else {
					logger.Warn("node IP skipped, please use .machine.kubelet.nodeIP to provide explicit subnet for the node IP", zap.Stringer("address", ip))
				}
			}
		}

		if err = safe.WriterModify(
			ctx,
			r,
			k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID),
			func(r *k8s.NodeIP) error {
				spec := r.TypedSpec()

				spec.Addresses = nodeIPs

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying NodeIP resource: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
