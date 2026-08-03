// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/partition"
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// wipedVolumeRediscoveryTimeout bounds the wait for block device discovery to catch up with the
// partition table after a wipe.
const wipedVolumeRediscoveryTimeout = 30 * time.Second

// VolumeWipeController reads the StagedWipeTargets META tag and wipes volumes matching its CEL selectors.
type VolumeWipeController struct {
	V1Alpha1Mode machinedruntime.Mode
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

		// in container mode there are no block devices to wipe, and the container boot sequence has no
		// 'meta' phase, so the reloadMeta task never runs and MetaLoaded is never created: waiting for
		// it below would block VolumeConfigController (which gates every non-META volume on
		// VolumeWipeStatus) for the whole boot.
		if ctrl.V1Alpha1Mode.InContainer() {
			return ctrl.signalWipeDone(ctx, r)
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
				logger.Info("waiting for discovered volumes to be ready (DiscoveredVolumesStatus not found)")

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
			return ctrl.signalWipeDone(ctx, r)
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

		var wiped []wipedVolume

		for _, selector := range selectors {
			var matchedVolume *block.DiscoveredVolume

			for _, vol := range volumes {
				matches, err := selector.EvalBool(volumeLocator, map[string]any{"volume": vol.spec})
				if err != nil {
					return fmt.Errorf("failed to evaluate wipe selector %q: %w", selector, err)
				}

				if matches {
					matchedVolume = vol.resource

					// Found a match
					break
				}
			}

			if matchedVolume == nil {
				logger.Sugar().Errorf("failed to execute staged wipe for volume %q: no matching volume", selector)

				continue
			}

			wipeTarget := partition.VolumeWipeTargetFromDiscoveredVolume(matchedVolume)

			// the API refuses to stage a META wipe, but a tag staged by an older version of Talos (or written
			// by hand) can still name it; wiping META here would drop a partition which is already provisioned
			// and in use for this boot, and nothing would reprovision it until the next reboot
			if wipeTarget.GetLabel() == constants.MetaPartitionLabel {
				logger.Sugar().Warnf("refusing staged wipe of %s: META can't be wiped", wipeTarget)

				continue
			}

			logger.Sugar().Infof("executing staged wipe of %s", wipeTarget)

			if err := wipeTarget.Wipe(ctx, logfn); err != nil {
				logger.Sugar().Errorf("failed wiping %s: %v", wipeTarget, err)

				continue
			}

			wiped = append(wiped, wipedVolume{
				id:            matchedVolume.Metadata().ID(),
				partitionUUID: matchedVolume.TypedSpec().PartitionUUID,
			})
		}

		// all wipes are done, but the volumes can't be provisioned until the discovery catches up
		if err := ctrl.waitForRediscovery(ctx, r, logger, wiped); err != nil {
			if ctx.Err() != nil {
				// shutting down
				return nil
			}

			return err
		}

		return ctrl.signalWipeDone(ctx, r)
	}
}

// wipedVolume identifies a volume which was wiped, so that the controller can tell whether block
// device discovery still reports it.
type wipedVolume struct {
	id            resource.ID
	partitionUUID string
}

// waitForRediscovery waits until block device discovery no longer reports the volumes which were just wiped.
//
// Wiping a volume drops its partition, but DiscoveredVolume resources are only refreshed once
// DiscoveryController observes the matching udev events. Signaling the wipe as complete while the
// discovery is still stale lets VolumeManagerController locate a volume at a device which no longer
// exists: the volume then fails to be provisioned, and as the volume state machine resumes a failed
// volume at its pre-failure phase, it never re-locates the volume, so e.g. STATE stays failed for
// the rest of the boot, and the machine configuration never loads.
func (ctrl *VolumeWipeController) waitForRediscovery(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	wiped []wipedVolume,
) error {
	if len(wiped) == 0 {
		return nil
	}

	timeout := time.After(wipedVolumeRediscoveryTimeout)

	// discovery is driven by udev events, which don't necessarily produce an event on our inputs, so poll as well
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		stale, err := ctrl.staleWipedVolumes(ctx, r, wiped)
		if err != nil {
			return err
		}

		if len(stale) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			// proceeding is not fatal (VolumeManagerController ignores discovered volumes whose device
			// is gone), but the discovery being this slow is worth a loud log line
			logger.Sugar().Errorf("timed out waiting for wiped volumes to be re-discovered: %v", stale)

			return nil
		case <-ticker.C:
		case <-r.EventCh():
		}
	}
}

// staleWipedVolumes returns the IDs of the wiped volumes which are still reported by the discovery.
func (ctrl *VolumeWipeController) staleWipedVolumes(ctx context.Context, r controller.Runtime, wiped []wipedVolume) ([]resource.ID, error) {
	var stale []resource.ID

	for _, volume := range wiped {
		discoveredVolume, err := safe.ReaderGetByID[*block.DiscoveredVolume](ctx, r, volume.id)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, fmt.Errorf("failed to get discovered volume %q: %w", volume.id, err)
		}

		// a partition provisioned in place of the wiped one reuses the device name, but gets a fresh partition UUID
		if discoveredVolume.TypedSpec().PartitionUUID == volume.partitionUUID {
			stale = append(stale, volume.id)
		}
	}

	return stale, nil
}

// signalWipeDone publishes VolumeWipeStatus, which VolumeConfigController waits for before creating
// any non-META volume.
func (ctrl *VolumeWipeController) signalWipeDone(ctx context.Context, r controller.Runtime) error {
	if err := safe.WriterModify(ctx, r, block.NewVolumeWipeStatus(block.NamespaceName, block.VolumeWipeID), func(status *block.VolumeWipeStatus) error {
		status.TypedSpec().Ready = true

		return nil
	}); err != nil {
		return fmt.Errorf("failed to write volume wipe status: %w", err)
	}

	return nil
}
