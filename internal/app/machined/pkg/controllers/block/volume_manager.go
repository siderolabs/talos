// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VolumeManagerController manages volumes in the system, converting VolumeConfig resources to VolumeStatuses.
type VolumeManagerController struct{}

// Name implements controller.Controller interface.
func (ctrl *VolumeManagerController) Name() string {
	return "block.VolumeManagerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VolumeManagerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeConfigType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiskType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.SystemDiskType,
			ID:        optional.Some(block.SystemDiskID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.DevicesStatusType,
			ID:        optional.Some(runtime.DevicesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveryRefreshStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.SystemInformationType,
			ID:        optional.Some(hardware.SystemInformationID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeLifecycleType,
			ID:        optional.Some(block.VolumeLifecycleID),
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VolumeManagerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: block.DiscoveryRefreshRequestType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *VolumeManagerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		deviceReadyObserved bool
		deviceReadyRequest  int
	)

	retryTicker := time.NewTicker(30 * time.Second)
	defer retryTicker.Stop()

	shouldRetry := false

	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		case <-retryTicker.C:
			if !shouldRetry {
				continue
			}

			shouldRetry = false
		}

		// if devices are not ready, we can't provision and locate most volumes
		devicesStatus, err := safe.ReaderGetByID[*runtime.DevicesStatus](ctx, r, runtime.DevicesID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching devices status: %w", err)
		}

		devicesReady := devicesStatus != nil && devicesStatus.TypedSpec().Ready

		if devicesReady && !deviceReadyObserved {
			deviceReadyObserved = true

			// udevd reports that devices are ready, now it's time to refresh the discovery volumes
			if err = safe.WriterModify(ctx, r, block.NewDiscoveryRefreshRequest(block.NamespaceName, block.RefreshID), func(drr *block.DiscoveryRefreshRequest) error {
				drr.TypedSpec().Request++
				deviceReadyRequest = drr.TypedSpec().Request

				return nil
			}); err != nil {
				return fmt.Errorf("error updating discovery refresh request: %w", err)
			}
		}

		refreshStatus, err := safe.ReaderGetByID[*block.DiscoveryRefreshStatus](ctx, r, block.RefreshID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching discovery refresh status: %w", err)
		}

		// now devicesReady is only true if the refresh status is up to date
		devicesReady = devicesReady && refreshStatus != nil && refreshStatus.TypedSpec().Request == deviceReadyRequest

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("error fetching discovered volumes: %w", err)
		}

		discoveredVolumesSpecs, err := safe.Map(discoveredVolumes, func(dv *block.DiscoveredVolume) (*blockpb.DiscoveredVolumeSpec, error) {
			spec := &blockpb.DiscoveredVolumeSpec{}

			return spec, proto.ResourceSpecToProto(dv, spec)
		})
		if err != nil {
			return fmt.Errorf("error mapping discovered volumes: %w", err)
		}

		disks, err := safe.ReaderListAll[*block.Disk](ctx, r)
		if err != nil {
			return fmt.Errorf("error fetching disks: %w", err)
		}

		systemDisk, err := safe.ReaderGetByID[*block.SystemDisk](ctx, r, block.SystemDiskID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching system disk: %w", err)
		}

		volumeLifecycle, err := safe.ReaderGetByID[*block.VolumeLifecycle](ctx, r, block.VolumeLifecycleID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching volume lifecycle: %w", err)
		}

		if volumeLifecycle == nil {
			// no volume lifecycle, cease all operations
			continue
		}

		if volumeLifecycle.Metadata().Phase() == resource.PhaseRunning {
			if err = r.AddFinalizer(ctx, volumeLifecycle.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer to volume lifecycle: %w", err)
			}
		}

		diskSpecs, err := safe.Map(disks, func(d *block.Disk) (volumes.DiskContext, error) {
			spec := &blockpb.DiskSpec{}

			if err := proto.ResourceSpecToProto(d, spec); err != nil {
				return volumes.DiskContext{}, err
			}

			var optionalSystemDisk optional.Optional[bool]

			if systemDisk != nil {
				optionalSystemDisk = optional.Some(d.Metadata().ID() == systemDisk.TypedSpec().DiskID)
			}

			return volumes.DiskContext{
				Disk:       spec,
				SystemDisk: optionalSystemDisk,
			}, nil
		})
		if err != nil {
			return fmt.Errorf("error mapping disks: %w", err)
		}

		volumeConfigList, err := safe.ReaderListAll[*block.VolumeConfig](ctx, r)
		if err != nil {
			return fmt.Errorf("error fetching volume configurations: %w", err)
		}

		volumeStatusList, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error fetching volume statuses: %w", err)
		}

		volumeConfigIDs := xslices.ToSet(safe.ToSlice(volumeConfigList, func(vc *block.VolumeConfig) resource.ID { return vc.Metadata().ID() }))

		volumeStatuses := xslices.ToMap(
			safe.ToSlice(volumeStatusList, func(vs *block.VolumeStatus) *block.VolumeStatus { return vs }),
			func(vs *block.VolumeStatus) (resource.ID, *block.VolumeStatusSpec) {
				return vs.Metadata().ID(), vs.TypedSpec()
			},
		)

		if volumeStatuses == nil {
			volumeStatuses = map[resource.ID]*block.VolumeStatusSpec{}
		}

		// ensure all volume configs have our finalizers
		for vc := range volumeConfigList.All() {
			if vc.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if vc.Metadata().Finalizers().Has(ctrl.Name()) {
				continue
			}

			if err = r.AddFinalizer(ctx, vc.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer to volume configuration: %w", err)
			}
		}

		// remove statuses for volume configs that no longer exist
		for id := range volumeStatuses {
			if _, exists := volumeConfigIDs[id]; !exists {
				delete(volumeStatuses, id)

				if err := r.Destroy(ctx, block.NewVolumeStatus(block.NamespaceName, id).Metadata()); err != nil {
					return fmt.Errorf("error destroying volume status: %w", err)
				}
			}
		}

		// fill in statuses for volume configs that don't have a status yet
		for id := range volumeConfigIDs {
			if _, exists := volumeStatuses[id]; !exists {
				volumeStatuses[id] = &block.VolumeStatusSpec{
					Phase: block.VolumePhaseWaiting,
				}
			}
		}

		volumeConfigs := safe.ToSlice(volumeConfigList, func(vc *block.VolumeConfig) *block.VolumeConfig { return vc })

		// re-sort volume configs by provisioning wave
		slices.SortStableFunc(volumeConfigs, volumes.CompareVolumeConfigs)

		fullyProvisionedWave := math.MaxInt
		allClosed := true

		for _, vc := range volumeConfigs {
			// abort on context cancel, as each volume processing might take a while
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			volumeStatus := volumeStatuses[vc.Metadata().ID()]
			volumeLogger := logger.With(zap.String("volume", vc.Metadata().ID()))

			if vc.Metadata().Phase() != resource.PhaseRunning {
				// [TODO]: handle me later
				continue
			}

			prevPhase := volumeStatus.Phase

			if err = ctrl.processVolumeConfig(
				ctx,
				volumeLogger,
				volumes.ManagerContext{
					Cfg:                     vc,
					Status:                  volumeStatus,
					DiscoveredVolumes:       discoveredVolumesSpecs,
					Disks:                   diskSpecs,
					DevicesReady:            devicesReady,
					PreviousWaveProvisioned: vc.TypedSpec().Provisioning.Wave <= fullyProvisionedWave,
					GetSystemInformation: func(ctx context.Context) (*hardware.SystemInformation, error) {
						systemInfo, err := safe.ReaderGetByID[*hardware.SystemInformation](ctx, r, hardware.SystemInformationID)
						if err != nil && !state.IsNotFoundError(err) {
							return nil, fmt.Errorf("error fetching system information: %w", err)
						}

						if systemInfo == nil {
							return nil, errors.New("system information not available")
						}

						return systemInfo, nil
					},
					Lifecycle: volumeLifecycle,
				},
			); err != nil {
				volumeStatus.PreFailPhase = volumeStatus.Phase
				volumeStatus.Phase = block.VolumePhaseFailed
				volumeStatus.ErrorMessage = err.Error()

				if xerrors.TagIs[volumes.Retryable](err) {
					shouldRetry = true
				}
			} else {
				volumeStatus.ErrorMessage = ""
				volumeStatus.PreFailPhase = block.VolumePhase(0)
			}

			if volumeStatus.Phase != block.VolumePhaseReady {
				fullyProvisionedWave = vc.TypedSpec().Provisioning.Wave - 1
			}

			if prevPhase != volumeStatus.Phase || err != nil {
				volumeLogger.Info("volume status", zap.String("phase", fmt.Sprintf("%s -> %s", prevPhase, volumeStatus.Phase)), zap.Error(err))
			}

			allClosed = allClosed && volumeStatus.Phase == block.VolumePhaseClosed
		}

		// update statuses
		for id, spec := range volumeStatuses {
			if err = safe.WriterModify(ctx, r, block.NewVolumeStatus(block.NamespaceName, id), func(vs *block.VolumeStatus) error {
				*vs.TypedSpec() = *spec

				return nil
			}); err != nil {
				return fmt.Errorf("error updating volume status: %w", err)
			}
		}

		// remove our finalizer if all volumes are closed
		if volumeLifecycle.Metadata().Phase() == resource.PhaseTearingDown && allClosed {
			if err = r.RemoveFinalizer(ctx, volumeLifecycle.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error removing finalizer from volume lifecycle: %w", err)
			}
		}
	}
}

// processVolumeConfig implements the volume configuration automata.
//
//	Initial -> { Waiting }  ----> { Missing } // volume is not found (by locator)
//			    |                  |
//			    v                  v
//			{ Located }  ---->  { Provisioned } // partition is ready, grown as needed
//	                              |
//							      v
//	                            { Prepared } // decrypted (if needed)
//	                              |
//	                              v
//	                            { Ready } // can be mounted
//
//nolint:gocyclo,cyclop
func (ctrl *VolumeManagerController) processVolumeConfig(ctx context.Context, logger *zap.Logger, volumeContext volumes.ManagerContext) error {
	prevPhase := volumeContext.Status.Phase

	closingPhase := volumeContext.Lifecycle.Metadata().Phase() == resource.PhaseTearingDown

	for {
		if !closingPhase {
			// normal state machine
			switch volumeContext.Status.Phase {
			case block.VolumePhaseReady:
				// nothing to do, ready
				return nil
			case block.VolumePhaseWaiting, block.VolumePhaseMissing:
				if err := volumes.LocateAndProvision(ctx, logger, volumeContext); err != nil {
					return err
				}
			case block.VolumePhaseLocated:
				// grow the partition if needed
				if err := volumes.Grow(ctx, logger, volumeContext); err != nil {
					return err
				}
			case block.VolumePhaseProvisioned:
				// decrypt/encrypt the volume
				if err := volumes.HandleEncryption(ctx, logger, volumeContext); err != nil {
					return err
				}
			case block.VolumePhasePrepared:
				// format the volume
				if err := volumes.Format(ctx, logger, volumeContext); err != nil {
					return err
				}
			case block.VolumePhaseFailed:
				// recover from the failure by restoring the pre-failure phase
				volumeContext.Status.Phase = volumeContext.Status.PreFailPhase
			case block.VolumePhaseClosed:
				// no progress, stop the loop
				return nil
			}
		} else {
			// closing state machine
			switch volumeContext.Status.Phase {
			case block.VolumePhaseReady, block.VolumePhasePrepared:
				if err := volumes.Close(ctx, logger, volumeContext); err != nil {
					return err
				}
			case block.VolumePhaseWaiting, block.VolumePhaseMissing, block.VolumePhaseLocated, block.VolumePhaseProvisioned:
				volumeContext.Status.Phase = block.VolumePhaseClosed
			case block.VolumePhaseClosed:
				// done
				return nil
			case block.VolumePhaseFailed:
				// recover from the failure by restoring the pre-failure phase
				volumeContext.Status.Phase = volumeContext.Status.PreFailPhase
			}
		}

		if volumeContext.Status.Phase == prevPhase {
			// doesn't progress, stop the loop
			return nil
		}

		select {
		// abort
		case <-ctx.Done():
			return nil
		default:
		}

		prevPhase = volumeContext.Status.Phase
	}
}
