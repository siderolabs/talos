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
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        optional.Some(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeStatusType,
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
//nolint:gocyclo,cyclop
func (ctrl *AddressFilterController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting config: %w", err)
		}

		nodeName, err := safe.ReaderGetByID[*k8s.Nodename](ctx, r, k8s.NodenameID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting nodename: %w", err)
		}

		var nodeStatus *k8s.NodeStatus

		if nodeName != nil {
			nodeStatus, err = safe.ReaderGetByID[*k8s.NodeStatus](ctx, r, nodeName.TypedSpec().Nodename)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting nodename: %w", err)
			}
		}

		r.StartTrackingOutputs()

		if cfg != nil {
			var podCIDRs, serviceCIDRs []netip.Prefix

			if cfg.Config().K8sNetworkConfig() != nil {
				k8sNetwork := cfg.Config().K8sNetworkConfig()

				podCIDRs = slices.Clone(k8sNetwork.PodCIDRs())
				serviceCIDRs = slices.Clone(k8sNetwork.ServiceCIDRs())

				if nodeStatus != nil {
					podCIDRs = slices.Concat(podCIDRs, nodeStatus.TypedSpec().PodCIDRs)
				}
			}

			if err = safe.WriterModify(ctx, r, network.NewNodeAddressFilter(network.NamespaceName, k8s.NodeAddressFilterNoK8s), func(r *network.NodeAddressFilter) error {
				r.TypedSpec().ExcludeSubnets = slices.Concat(podCIDRs, serviceCIDRs)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating output resource: %w", err)
			}

			if err = safe.WriterModify(ctx, r, network.NewNodeAddressFilter(network.NamespaceName, k8s.NodeAddressFilterOnlyK8s), func(r *network.NodeAddressFilter) error {
				if len(podCIDRs)+len(serviceCIDRs) > 0 {
					r.TypedSpec().IncludeSubnets = slices.Concat(podCIDRs, serviceCIDRs)
					r.TypedSpec().ExcludeSubnets = nil
				} else {
					// if k8s is disabled, we want to exclude all networks from "k8s-only" filter
					r.TypedSpec().IncludeSubnets = nil
					r.TypedSpec().ExcludeSubnets = []netip.Prefix{
						netip.MustParsePrefix("0.0.0.0/0"),
						netip.MustParsePrefix("::/0"),
					}
				}

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
