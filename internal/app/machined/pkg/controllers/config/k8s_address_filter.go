// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/k8s"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// K8sAddressFilterController creates NodeAddressFilters based on machine configuration.
type K8sAddressFilterController struct{}

// Name implements controller.Controller interface.
func (ctrl *K8sAddressFilterController) Name() string {
	return "network.K8sAddressFilterController"
}

// Inputs implements controller.Controller interface.
func (ctrl *K8sAddressFilterController) Inputs() []controller.Input {
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
func (ctrl *K8sAddressFilterController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.NodeAddressFilterType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *K8sAddressFilterController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting config: %w", err)
		}

		touchedIDs := make(map[resource.ID]struct{})

		if cfg != nil {
			cfgProvider := cfg.(*config.MachineConfig).Config()

			var podCIDRs, serviceCIDRs []netaddr.IPPrefix

			for _, cidr := range cfgProvider.Cluster().Network().PodCIDRs() {
				var ipPrefix netaddr.IPPrefix

				ipPrefix, err = netaddr.ParseIPPrefix(cidr)
				if err != nil {
					return fmt.Errorf("error parsing podCIDR: %w", err)
				}

				podCIDRs = append(podCIDRs, ipPrefix)
			}

			for _, cidr := range cfgProvider.Cluster().Network().ServiceCIDRs() {
				var ipPrefix netaddr.IPPrefix

				ipPrefix, err = netaddr.ParseIPPrefix(cidr)
				if err != nil {
					return fmt.Errorf("error parsing serviceCIDR: %w", err)
				}

				serviceCIDRs = append(serviceCIDRs, ipPrefix)
			}

			if err = r.Modify(ctx, network.NewNodeAddressFilter(network.NamespaceName, k8s.NodeAddressFilterNoK8s), func(r resource.Resource) error {
				spec := r.(*network.NodeAddressFilter).TypedSpec()

				spec.ExcludeSubnets = append(append([]netaddr.IPPrefix(nil), podCIDRs...), serviceCIDRs...)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating output resource: %w", err)
			}

			touchedIDs[k8s.NodeAddressFilterNoK8s] = struct{}{}

			if err = r.Modify(ctx, network.NewNodeAddressFilter(network.NamespaceName, k8s.NodeAddressFilterOnlyK8s), func(r resource.Resource) error {
				spec := r.(*network.NodeAddressFilter).TypedSpec()

				spec.IncludeSubnets = append(append([]netaddr.IPPrefix(nil), podCIDRs...), serviceCIDRs...)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating output resource: %w", err)
			}

			touchedIDs[k8s.NodeAddressFilterOnlyK8s] = struct{}{}
		}

		// list keys for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressFilterType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}
	}
}
