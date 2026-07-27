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
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VolumeWipeController reads the StagedWipeTargets META tag and wipes volumes matching its CEL selectors.
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
			ID:        optional.Some(runtime.MetaKeyTagToID(meta.StagedWipeSelectors)),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumesStatusType,
			ID:        optional.Some(block.DiscoveredVolumesStatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaLoadedType,
			ID:        optional.Some(runtime.MetaLoadedID),
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
// TODO(majabojarska): once fully functional and with test cov, refactor this to bring down cyclo, and then remove the nolint directive.
//
//nolint:gocyclo,cyclop
func (ctrl *VolumeWipeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		metaLoaded, err := safe.ReaderGetByID[*runtime.MetaLoaded](ctx, r, runtime.MetaLoadedID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting meta loaded resource: %w", err)
		}

		if !metaLoaded.TypedSpec().Done {
			logger.Info("waiting for META to be loaded")

			continue
		}

		// At this point, we know META exists and has been loaded

		discoveredVolumesStatus, err := safe.ReaderGetByID[*block.DiscoveredVolumesStatus](ctx, r, block.DiscoveredVolumesStatusID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting discovered volumes status: %w", err)
		}

		if !discoveredVolumesStatus.TypedSpec().Ready {
			logger.Info("waiting for discovered volumes to be ready")

			continue
		}

		// Volumes are now ready, we can execute the wipe instructions.

		metaKey, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.StagedWipeSelectors))
		if err != nil && !state.IsNotFoundError(err) {
			// NotFound just means the tag is absent, which is a valid case (nothing to wipe).
			// We have checked that META is loaded, so any other error is unexpected.
			return fmt.Errorf("failed to read META key (StagedWipeTargets): %w", err)
		}

		if metaKey == nil {
			// Nothing to wipe: the StagedWipeTargets tag is absent.
			if err := safe.WriterModify(
				ctx,
				r,
				block.NewVolumeWipeStatus(block.NamespaceName, block.VolumeWipeID),
				func(status *block.VolumeWipeStatus) error {
					status.TypedSpec().Ready = true

					return nil
				}); err != nil {
				return fmt.Errorf("failed to write volume wipe status: %w", err)
			}

			return nil
		}

		// delete + flush the tag FIRST, before wiping — preserves safety (a wipe failure can't cause a boot loop)
		// and avoids re-triggering on the same generation
		if _, err := ctrl.MetaProvider.Meta().DeleteTag(ctx, meta.StagedWipeSelectors); err != nil {
			return fmt.Errorf("failed to delete staged wipe targets tag: %w", err)
		}

		if err := ctrl.MetaProvider.Meta().Flush(); err != nil {
			return fmt.Errorf("failed to flush meta: %w", err)
		}

		// unmarshal the stored CEL selectors
		var selectors []cel.Expression
		if err := json.Unmarshal([]byte(metaKey.TypedSpec().Value), &selectors); err != nil {
			return fmt.Errorf("failed to decode staged wipe selectors: %w", err)
		}

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		// convert each discovered volume to its proto spec once, for CEL evaluation
		type discoveredVolume struct {
			resource *block.DiscoveredVolume
			spec     *blockpb.DiscoveredVolumeSpec
		}

		volumes := make([]discoveredVolume, 0, discoveredVolumes.Len())

		for dv := range discoveredVolumes.All() {
			spec := &blockpb.DiscoveredVolumeSpec{}
			if err := proto.ResourceSpecToProto(dv, spec); err != nil {
				return fmt.Errorf("failed to convert discovered volume %q to proto: %w", dv.Metadata().ID(), err)
			}

			volumes = append(volumes, discoveredVolume{resource: dv, spec: spec})
		}

		logfn := func(format string, args ...any) {
			logger.Sugar().Infof(format, args...)
		}

		volumeLocator := celenv.VolumeLocator()

		for _, selector := range selectors {
			var wipeTarget *partition.VolumeWipeTarget

			for _, vol := range volumes {
				matches, err := selector.EvalBool(volumeLocator, map[string]any{"volume": vol.spec})
				if err != nil {
					return fmt.Errorf("failed to evaluate wipe selector %q: %w", selector, err)
				}

				if matches {
					wipeTarget = partition.VolumeWipeTargetFromDiscoveredVolume(vol.resource)

					// Found a match
					break
				}
			}

			if wipeTarget == nil {
				logger.Sugar().Errorf("failed to execute staged wipe for volume %q: no matching volume", selector)

				continue
			}

			logger.Sugar().Infof("executing staged wipe of %s", wipeTarget)

			if err := wipeTarget.Wipe(ctx, logfn); err != nil {
				logger.Sugar().Errorf("failed wiping %s: %v", wipeTarget, err)

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
