// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// BuildVolumeResources builds the volume resources in the order they are to be published.
func BuildVolumeResources(
	ctx context.Context, cfg configconfig.Config, encryptionMeta *runtime.MetaKey, inContainer, isAgent bool,
) ([]VolumeResource, error) {
	transformers := append(
		GetSystemVolumeTransformers(ctx, encryptionMeta, inContainer, isAgent),
		UserVolumeTransformers...,
	)

	var resources []VolumeResource

	for _, transformer := range transformers {
		transformed, err := transformer(cfg)
		if err != nil {
			return nil, err
		}

		resources = append(resources, transformed...)
	}

	// each volume config is published separately, so VolumeManagerController may pick up any prefix
	// of this sequence. Publishing in provisioning order makes every such prefix a valid provisioning
	// prefix, so a volume is never provisioned ahead of the volumes which should come first
	// (e.g. EPHEMERAL grows to fill the disk, and must never be provisioned before the promotable
	// system volumes which share the system disk with it in the same wave).
	return sortByProvisioningOrder(resources)
}

// sortByProvisioningOrder sorts the volume resources into the order VolumeManagerController
// provisions them, i.e. by [volumes.CompareVolumeConfigs].
//
// The provisioning order is a property of the resulting VolumeConfig spec, so each resource is
// transformed into a throwaway VolumeConfig to figure out where it belongs. This is safe as the
// transform functions only set fields on the spec: they never read the previous spec, and never
// touch the metadata.
func sortByProvisioningOrder(resources []VolumeResource) ([]VolumeResource, error) {
	specs := make(map[string]*block.VolumeConfig, len(resources))

	for _, rsrc := range resources {
		volumeConfig := block.NewVolumeConfig(block.NamespaceName, rsrc.VolumeID)

		if err := rsrc.TransformFunc(volumeConfig); err != nil {
			return nil, fmt.Errorf("error building volume config %s: %w", rsrc.VolumeID, err)
		}

		specs[rsrc.VolumeID] = volumeConfig
	}

	sorted := slices.Clone(resources)

	slices.SortStableFunc(sorted, func(a, b VolumeResource) int {
		return volumes.CompareVolumeConfigs(specs[a.VolumeID], specs[b.VolumeID])
	})

	return sorted, nil
}
