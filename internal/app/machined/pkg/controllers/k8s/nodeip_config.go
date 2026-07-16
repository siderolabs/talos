// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// NodeIPConfigController configures k8s.NodeIP based on machine config.
type NodeIPConfigController = transform.Controller[*config.MachineConfig, *k8s.NodeIPConfig]

// NewNodeIPConfigController instantiates the controller.
//
//nolint:gocyclo,dupl
func NewNodeIPConfigController() *NodeIPConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.NodeIPConfig]{
			Name: "k8s.NodeIPConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*k8s.NodeIPConfig] { //nolint:dupl
				if cfg.Metadata().ID() != config.ActiveID {
					return optional.None[*k8s.NodeIPConfig]()
				}

				if cfg.Config().Machine() == nil {
					return optional.None[*k8s.NodeIPConfig]()
				}

				if cfg.Config().K8sNetworkConfig() == nil {
					return optional.None[*k8s.NodeIPConfig]()
				}

				if cfg.Config().K8sNodeConfig() == nil {
					return optional.None[*k8s.NodeIPConfig]()
				}

				return optional.Some(k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *k8s.NodeIPConfig) error {
				spec := res.TypedSpec()
				cfgProvider := cfg.Config()

				spec.ValidSubnets = cfgProvider.K8sNodeConfig().NodeIP().ValidSubnets()

				if len(spec.ValidSubnets) == 0 {
					// automatically deduce validsubnets from ServiceCIDRs
					spec.ValidSubnets = ipSubnetsFromServiceCIDRs(cfgProvider.K8sNetworkConfig().ServiceCIDRs())
				}

				spec.ExcludeSubnets = nil

				// filter out Pod & Service CIDRs, they can't be kubelet IPs
				spec.ExcludeSubnets = append(
					append(
						spec.ExcludeSubnets,
						xslices.Map(cfgProvider.K8sNetworkConfig().PodCIDRs(), netip.Prefix.String)...,
					),
					xslices.Map(cfgProvider.K8sNetworkConfig().ServiceCIDRs(), netip.Prefix.String)...,
				)

				// filter out any virtual IPs, they can't be node IPs either
				for _, device := range cfgProvider.Machine().Network().Devices() {
					if device.VIPConfig() != nil {
						spec.ExcludeSubnets = append(spec.ExcludeSubnets, device.VIPConfig().IP())
					}

					for _, vlan := range device.Vlans() {
						if vlan.VIPConfig() != nil {
							spec.ExcludeSubnets = append(spec.ExcludeSubnets, vlan.VIPConfig().IP())
						}
					}
				}

				for _, doc := range cfgProvider.NetworkVirtualIPConfigs() {
					spec.ExcludeSubnets = append(spec.ExcludeSubnets, doc.VIP().String())
				}

				return nil
			},
		},
	)
}

func ipSubnetsFromServiceCIDRs(serviceCIDRs []netip.Prefix) []string {
	// automatically configure valid IP subnets based on service CIDRs
	// if the primary service CIDR is IPv4, primary kubelet node IP should be IPv4 as well, and so on
	result := make([]string, 0, len(serviceCIDRs))

	for _, network := range serviceCIDRs {
		if network.Addr().Is6() {
			result = append(result, "::/0")
		} else {
			result = append(result, "0.0.0.0/0")
		}
	}

	return result
}
