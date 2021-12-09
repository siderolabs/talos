// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cluster provides controllers which manage Talos cluster resources.
package cluster

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
)

func cleanupAffiliates(ctx context.Context, ctrl controller.Controller, r controller.Runtime, touchedIDs map[resource.ID]struct{}) error {
	// list keys for cleanup
	list, err := r.List(ctx, resource.NewMetadata(cluster.RawNamespaceName, cluster.AffiliateType, "", resource.VersionUndefined))
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

	return nil
}
