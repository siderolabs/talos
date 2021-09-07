// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/resources/cluster"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/kubespan"
)

// PeerSpecController watches cluster.Affiliates updates PeerSpec.
type PeerSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *PeerSpecController) Name() string {
	return "kubespan.PeerSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PeerSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      kubespan.ConfigType,
			ID:        pointer.ToString(kubespan.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.AffiliateType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        pointer.ToString(cluster.LocalIdentity),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PeerSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: kubespan.PeerSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *PeerSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, kubespan.ConfigType, kubespan.ConfigID, resource.VersionUndefined))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting kubespan configuration: %w", err)
			}

			localIdentity, err := r.Get(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.IdentityType, cluster.LocalIdentity, resource.VersionUndefined))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting first MAC address: %w", err)
			}

			affiliates, err := r.List(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.AffiliateType, "", resource.VersionUndefined))
			if err != nil {
				return fmt.Errorf("error listing cluster affiliates: %w", err)
			}

			touchedIDs := make(map[resource.ID]struct{})

			if cfg != nil && localIdentity != nil && cfg.(*kubespan.Config).TypedSpec().Enabled {
				localAffiliateID := localIdentity.(*cluster.Identity).TypedSpec().NodeID

				for _, affiliate := range affiliates.Items {
					if affiliate.Metadata().ID() == localAffiliateID {
						// skip local affiliate, it's not a peer
						continue
					}

					spec := affiliate.(*cluster.Affiliate).TypedSpec()

					if spec.KubeSpan.PublicKey == "" {
						// no kubespan information, skip it
						continue
					}

					if err = r.Modify(ctx, kubespan.NewPeerSpec(kubespan.NamespaceName, spec.KubeSpan.PublicKey), func(res resource.Resource) error {
						*res.(*kubespan.PeerSpec).TypedSpec() = kubespan.PeerSpecSpec{
							Address:             spec.KubeSpan.Address,
							AdditionalAddresses: append([]netaddr.IPPrefix(nil), spec.KubeSpan.AdditionalAddresses...),
							Endpoints:           append([]netaddr.IPPort(nil), spec.KubeSpan.Endpoints...),
							Label:               spec.Nodename,
						}

						return nil
					}); err != nil {
						return err
					}

					touchedIDs[spec.KubeSpan.PublicKey] = struct{}{}
				}
			}

			// list keys for cleanup
			list, err := r.List(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.PeerSpecType, "", resource.VersionUndefined))
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
}
