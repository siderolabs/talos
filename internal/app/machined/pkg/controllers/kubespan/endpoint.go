// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/value"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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
			Namespace: config.NamespaceName,
			Type:      kubespan.ConfigType,
			ID:        optional.Some(kubespan.ConfigID),
			Kind:      controller.InputWeak,
		},
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
func (ctrl *EndpointController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*kubespan.Config](ctx, r, kubespan.ConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting kubespan configuration: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg == nil || !cfg.TypedSpec().HarvestExtraEndpoints {
			// not enabled, short-circuit early
			if err = safe.CleanupOutputs[*kubespan.Endpoint](ctx, r); err != nil {
				return err
			}

			continue
		}

		// for every kubespan peer, if it's up and has endpoint, harvest that endpoint
		peerStatuses, err := safe.ReaderListAll[*kubespan.PeerStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing cluster affiliates: %w", err)
		}

		affiliates, err := safe.ReaderListAll[*cluster.Affiliate](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing cluster affiliates: %w", err)
		}

		// build lookup table of affiliate's kubespan public key back to affiliate ID
		affiliateLookup := make(map[string]string)

		for affiliate := range affiliates.All() {
			affiliateSpec := affiliate.TypedSpec()

			if affiliateSpec.KubeSpan.PublicKey != "" {
				affiliateLookup[affiliateSpec.KubeSpan.PublicKey] = affiliateSpec.NodeID
			}
		}

		for res := range peerStatuses.All() {
			peerStatus := res.TypedSpec()

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

			if err = safe.WriterModify(ctx, r, kubespan.NewEndpoint(kubespan.NamespaceName, res.Metadata().ID()), func(res *kubespan.Endpoint) error {
				*res.TypedSpec() = kubespan.EndpointSpec{
					AffiliateID: affiliateID,
					Endpoint:    peerStatus.Endpoint,
				}

				return nil
			}); err != nil {
				return err
			}
		}

		if err = safe.CleanupOutputs[*kubespan.Endpoint](ctx, r); err != nil {
			return err
		}
	}
}
