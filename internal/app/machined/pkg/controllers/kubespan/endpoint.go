// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/value"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

// EndpointController watches KubeSpanPeerStatuses, Affiliates and harvests additional endpoints for the peers.
type EndpointController struct{}

// Name implements controller.Controller interface.
func (ctrl *EndpointController) Name() string {
	return "kubespan.EndpointController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EndpointController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.AffiliateType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: kubespan.NamespaceName,
			Type:      kubespan.PeerStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EndpointController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: kubespan.EndpointType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EndpointController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		peerStatuses, err := r.List(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.PeerStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing cluster affiliates: %w", err)
		}

		affiliates, err := r.List(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.AffiliateType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing cluster affiliates: %w", err)
		}

		// build lookup table of affiliate's kubespan public key back to affiliate ID
		affiliateLookup := make(map[string]string)

		for _, res := range affiliates.Items {
			affiliate := res.(*cluster.Affiliate).TypedSpec()

			if affiliate.KubeSpan.PublicKey != "" {
				affiliateLookup[affiliate.KubeSpan.PublicKey] = affiliate.NodeID
			}
		}

		// for every kubespan peer, if it's up and has endpoint, harvest that endpoint
		touchedIDs := make(map[resource.ID]struct{})

		for _, res := range peerStatuses.Items {
			peerStatus := res.(*kubespan.PeerStatus).TypedSpec()

			if peerStatus.State != kubespan.PeerStateUp {
				continue
			}

			if value.IsZero(peerStatus.Endpoint) {
				continue
			}

			affiliateID, ok := affiliateLookup[res.Metadata().ID()]
			if !ok {
				continue
			}

			if err = r.Modify(ctx, kubespan.NewEndpoint(kubespan.NamespaceName, res.Metadata().ID()), func(res resource.Resource) error {
				*res.(*kubespan.Endpoint).TypedSpec() = kubespan.EndpointSpec{
					AffiliateID: affiliateID,
					Endpoint:    peerStatus.Endpoint,
				}

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[res.Metadata().ID()] = struct{}{}
		}

		// list keys for cleanup
		list, err := r.List(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.EndpointType, "", resource.VersionUndefined))
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

		r.ResetRestartBackoff()
	}
}
