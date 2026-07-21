// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VolumeWipeController reads the StagedPartitionsToWipe META tag and wipes volumes by UUID.
type VolumeWipeController struct {
	MetaProvider volumeconfig.MetaProvider
}

// Name implements controller.Controller interface.
func (ctrl *VolumeWipeController) Name() string {
	return "block.VolumeWipeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VolumeWipeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaKeyType,
			ID:        optional.Some(runtime.MetaKeyTagToID(meta.StagedPartitionsToWipe)),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VolumeWipeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeWipeStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
// TODO(majabojarska): refactor this to bring down cyclo
//
//nolint:gocyclo
func (ctrl *VolumeWipeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		metaKey, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.StagedPartitionsToWipe))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to get staged partitions to wipe meta key: %w", err)
		}

		if metaKey == nil {
			continue
		}

		// unmarshal the stored partition UUIDs
		var partitionUUIDs map[string]bool
		if err := json.Unmarshal([]byte(metaKey.TypedSpec().Value), &partitionUUIDs); err != nil {
			return fmt.Errorf("failed to decode staged partitions to wipe tag: %w", err)
		}

		// delete + flush the tag FIRST, before wiping — preserves safety (a wipe failure can't cause a boot loop)
		// and avoids re-triggering on the same generation
		if _, err := ctrl.MetaProvider.Meta().DeleteTag(ctx, meta.StagedPartitionsToWipe); err != nil {
			return fmt.Errorf("failed to delete staged partitions to wipe tag: %w", err)
		}

		if err := ctrl.MetaProvider.Meta().Flush(); err != nil {
			return fmt.Errorf("failed to flush meta: %w", err)
		}

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		logfn := func(format string, args ...any) {
			logger.Sugar().Infof(format, args...)
		}

		for uuid := range partitionUUIDs {
			var wipeTarget *partition.VolumeWipeTarget

			for dv := range discoveredVolumes.All() {
				if dv.TypedSpec().PartitionUUID == uuid {
					wipeTarget = partition.VolumeWipeTargetFromDiscoveredVolume(dv)

					// Found a match
					break
				}
			}

			if wipeTarget == nil {
				logger.Sugar().Infof("skipping staged wipe of partition %q: not found", uuid)

				continue
			}

			logger.Sugar().Infof("executing staged wipe of partition %s", wipeTarget)

			if err := wipeTarget.Wipe(ctx, logfn); err != nil {
				logger.Sugar().Errorf("failed wiping partition %s: %w", wipeTarget, err)

				continue
			}
		}

		// All wipes are done, write a VolumeWipeStatus resource to signal that the wipe is complete.
		if err := safe.WriterModify(ctx, r, block.NewVolumeWipeStatus(block.NamespaceName, block.VolumeWipeID), func(status *block.VolumeWipeStatus) error {
			status.TypedSpec().Ready = true

			return nil
		}); err != nil {
			return fmt.Errorf("failed to write volume wipe status: %w", err)
		}
	}
}
