// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

// AffiliateMergeController merges raw Affiliates from the RawNamespaceName into final representation in the NamespaceName.
type AffiliateMergeController struct{}

// Name implements controller.Controller interface.
func (ctrl *AffiliateMergeController) Name() string {
	return "cluster.AffiliateMergeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AffiliateMergeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: cluster.RawNamespaceName,
			Type:      cluster.AffiliateType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *AffiliateMergeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.AffiliateType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *AffiliateMergeController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		rawAffiliates, err := safe.ReaderList[*cluster.Affiliate](ctx, r, resource.NewMetadata(cluster.RawNamespaceName, cluster.AffiliateType, "", resource.VersionUndefined))
		if err != nil {
			return errors.New("error listing affiliates")
		}

		mergedAffiliates := make(map[resource.ID]*cluster.AffiliateSpec, rawAffiliates.Len())

		for rawAffiliate := range rawAffiliates.All() {
			affiliateSpec := rawAffiliate.TypedSpec()
			id := affiliateSpec.NodeID

			if affiliate, ok := mergedAffiliates[id]; ok {
				affiliate.Merge(affiliateSpec)
			} else {
				mergedAffiliates[id] = affiliateSpec
			}
		}

		touchedIDs := make(map[resource.ID]struct{}, len(mergedAffiliates))

		for id, affiliateSpec := range mergedAffiliates {
			if err = safe.WriterModify(ctx, r, cluster.NewAffiliate(cluster.NamespaceName, id), func(res *cluster.Affiliate) error {
				*res.TypedSpec() = *affiliateSpec

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[id] = struct{}{}
		}

		// list keys for cleanup
		list, err := safe.ReaderListAll[*cluster.Affiliate](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for res := range list.All() {
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
