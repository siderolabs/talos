// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// NodeIPConfigController configures k8s.NodeIP based on machine config.
type NodeIPConfigController = transform.Controller[*config.MachineConfig, *k8s.NodeIPConfig]

// NewNodeIPConfigController instanciates the controller.
func NewNodeIPConfigController() *NodeIPConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.NodeIPConfig]{
			Name: "k8s.NodeIPConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*k8s.NodeIPConfig] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*k8s.NodeIPConfig]()
				}

				if cfg.Config().Machine() == nil || cfg.Config().Cluster() == nil {
					return optional.None[*k8s.NodeIPConfig]()
				}

				return optional.Some(k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *k8s.NodeIPConfig) error {
				spec := res.TypedSpec()
				cfgProvider := cfg.Config()

				spec.ValidSubnets = cfgProvider.Machine().Kubelet().NodeIP().ValidSubnets()

				if len(spec.ValidSubnets) == 0 {
					// automatically deduce validsubnets from ServiceCIDRs
					var err error

					spec.ValidSubnets, err = ipSubnetsFromServiceCIDRs(cfgProvider.Cluster().Network().ServiceCIDRs())
					if err != nil {
						return fmt.Errorf("error building valid subnets: %w", err)
					}
				}

				spec.ExcludeSubnets = nil

				// filter out Pod & Service CIDRs, they can't be kubelet IPs
				spec.ExcludeSubnets = append(
					append(
						spec.ExcludeSubnets,
						cfgProvider.Cluster().Network().PodCIDRs()...,
					),
					cfgProvider.Cluster().Network().ServiceCIDRs()...,
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

				return nil
			},
		},
	)
}

func ipSubnetsFromServiceCIDRs(serviceCIDRs []string) ([]string, error) {
	// automatically configure valid IP subnets based on service CIDRs
	// if the primary service CIDR is IPv4, primary kubelet node IP should be IPv4 as well, and so on
	result := make([]string, 0, len(serviceCIDRs))

	for _, cidr := range serviceCIDRs {
		network, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subnet: %w", err)
		}

		if network.Addr().Is6() {
			result = append(result, "::/0")
		} else {
			result = append(result, "0.0.0.0/0")
		}
	}

	return result, nil
}
