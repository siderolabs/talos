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

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// NodeIPConfigController renders manifests based on templates and config/secrets.
type NodeIPConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeIPConfigController) Name() string {
	return "k8s.NodeIPConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeIPConfigController) Inputs() []controller.Input {
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
func (ctrl *NodeIPConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodeIPConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeIPConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.(*config.MachineConfig).Config()

		if err = r.Modify(
			ctx,
			k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID),
			func(r resource.Resource) error {
				spec := r.(*k8s.NodeIPConfig).TypedSpec()

				spec.ValidSubnets = cfgProvider.Machine().Kubelet().NodeIP().ValidSubnets()

				if len(spec.ValidSubnets) == 0 {
					// automatically deduce validsubnets from ServiceCIDRs
					spec.ValidSubnets, err = ipSubnetsFromServiceCIDRs(cfgProvider.Cluster().Network().ServiceCIDRs())
					if err != nil {
						return fmt.Errorf("error building valid subnets: %w", err)
					}
				}

				spec.ExcludeSubnets = nil

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
		); err != nil {
			return fmt.Errorf("error modifying NodeIPConfig resource: %w", err)
		}
	}
}

func ipSubnetsFromServiceCIDRs(serviceCIDRs []string) ([]string, error) {
	// automatically configure valid IP subnets based on service CIDRs
	// if the primary service CIDR is IPv4, primary kubelet node IP should be IPv4 as well, and so on
	result := make([]string, 0, len(serviceCIDRs))

	for _, cidr := range serviceCIDRs {
		network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subnet: %w", err)
		}

		if network.IP.To4() == nil {
			result = append(result, "::/0")
		} else {
			result = append(result, "0.0.0.0/0")
		}
	}

	return result, nil
}
