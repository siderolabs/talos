// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// AddressFilterController creates NodeAddressFilters based on machine configuration.
type AddressFilterController struct{}

// Name implements controller.Controller interface.
func (ctrl *AddressFilterController) Name() string {
	return "k8s.AddressFilterController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AddressFilterController) Inputs() []controller.Input {
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
func (ctrl *AddressFilterController) Outputs() []controller.Output {
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
func (ctrl *AddressFilterController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting config: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg != nil && cfg.Config().Cluster() != nil {
			cfgProvider := cfg.Config()

			var podCIDRs, serviceCIDRs []netip.Prefix

			for _, cidr := range cfgProvider.Cluster().Network().PodCIDRs() {
				var ipPrefix netip.Prefix

				ipPrefix, err = netip.ParsePrefix(cidr)
				if err != nil {
					return fmt.Errorf("error parsing podCIDR: %w", err)
				}

				podCIDRs = append(podCIDRs, ipPrefix)
			}

			for _, cidr := range cfgProvider.Cluster().Network().ServiceCIDRs() {
				var ipPrefix netip.Prefix

				ipPrefix, err = netip.ParsePrefix(cidr)
				if err != nil {
					return fmt.Errorf("error parsing serviceCIDR: %w", err)
				}

				serviceCIDRs = append(serviceCIDRs, ipPrefix)
			}

			if err = safe.WriterModify(ctx, r, network.NewNodeAddressFilter(network.NamespaceName, k8s.NodeAddressFilterNoK8s), func(r *network.NodeAddressFilter) error {
				r.TypedSpec().ExcludeSubnets = append(slices.Clone(podCIDRs), serviceCIDRs...)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating output resource: %w", err)
			}

			if err = safe.WriterModify(ctx, r, network.NewNodeAddressFilter(network.NamespaceName, k8s.NodeAddressFilterOnlyK8s), func(r *network.NodeAddressFilter) error {
				r.TypedSpec().IncludeSubnets = append(slices.Clone(podCIDRs), serviceCIDRs...)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating output resource: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*network.NodeAddressFilter](ctx, r); err != nil {
			return err
		}
	}
}
