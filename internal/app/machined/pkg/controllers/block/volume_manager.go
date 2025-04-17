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
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
			Type:      block.VolumeStatusType,
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
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.PCRStatusType,
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

		volumeStatuses := xslices.ToMap(
			safe.ToSlice(volumeStatusList, func(vs *block.VolumeStatus) *block.VolumeStatus { return vs }),
			func(vs *block.VolumeStatus) (resource.ID, *block.VolumeStatus) {
				return vs.Metadata().ID(), vs
			},
		)

		if volumeStatuses == nil {
			volumeStatuses = map[resource.ID]*block.VolumeStatus{}
		}

		// ensure all volume configs have our finalizers
		for vc := range volumeConfigList.All() {
			if vc.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if !vc.Metadata().Finalizers().Has(ctrl.Name()) {
				if err = r.AddFinalizer(ctx, vc.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error adding finalizer to volume configuration: %w", err)
				}
			}
		}

		volumeLifecycleTearingDown := volumeLifecycle.Metadata().Phase() == resource.PhaseTearingDown

		if volumeLifecycleTearingDown {
			for _, fin := range *volumeLifecycle.Metadata().Finalizers() {
				if fin == ctrl.Name() {
					continue
				}

				// if there are finalizers other than us, we don't start global teardown
				volumeLifecycleTearingDown = false

				break
			}
		}

		volumeConfigs := safe.ToSlice(volumeConfigList, identity)

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

			var volumeParentStatus *block.VolumeStatus

			if vc.TypedSpec().ParentID != "" {
				volumeParentStatus = volumeStatuses[vc.TypedSpec().ParentID]
			}

			parentFinalizer := ctrl.Name() + "-" + vc.Metadata().ID()

			// figure out if we are tearing down this volume or building it
			tearingDown := (volumeStatus != nil && volumeStatus.Metadata().Phase() == resource.PhaseTearingDown) || // we started tearing down the volume, so finish doing so
				vc.Metadata().Phase() == resource.PhaseTearingDown || // volume config is being torn down
				volumeParentStatus != nil && volumeParentStatus.Metadata().Phase() == resource.PhaseTearingDown || // parent volume is being torn down
				volumeLifecycleTearingDown // global volume lifecycle requires all volumes to be torn down

			// volume status doesn't exist yet, figure out what to do
			if volumeStatus == nil {
				if tearingDown {
					if volumeParentStatus != nil {
						if volumeParentStatus.Metadata().Finalizers().Has(parentFinalizer) {
							if err = r.RemoveFinalizer(ctx, volumeParentStatus.Metadata(), parentFinalizer); err != nil {
								return fmt.Errorf("error removing finalizer from parent volume configuration: %w", err)
							}
						}
					}

					// happy case, we don't need to progress this volume
					if vc.Metadata().Finalizers().Has(ctrl.Name()) {
						if err = r.RemoveFinalizer(ctx, vc.Metadata(), ctrl.Name()); err != nil {
							return fmt.Errorf("error removing finalizer from volume configuration: %w", err)
						}
					}

					continue
				}

				// create a stub volume status
				volumeStatus = block.NewVolumeStatus(block.NamespaceName, vc.Metadata().ID())

				for k, v := range vc.Metadata().Labels().Raw() {
					volumeStatus.Metadata().Labels().Set(k, v)
				}

				volumeStatus.TypedSpec().Phase = block.VolumePhaseWaiting
				volumeStatus.TypedSpec().Type = vc.TypedSpec().Type
				volumeStatus.TypedSpec().ParentID = vc.TypedSpec().ParentID

				volumeStatuses[vc.Metadata().ID()] = volumeStatus
			}

			if tearingDown && volumeStatus.Metadata().Phase() != resource.PhaseTearingDown {
				// volume status is not yet in the tearing down phase, so move it there
				_, err = r.Teardown(ctx, volumeStatus.Metadata())
				if err != nil {
					return fmt.Errorf("error tearing down volume status: %w", err)
				}
			}

			shouldCloseVolume := tearingDown && volumeStatus.Metadata().Finalizers().Empty() // we can start closing volume as soon as all finalizers are gone, so the volume is not e.g. mounted

			prevPhase := volumeStatus.TypedSpec().Phase

			if err = ctrl.progressVolumeConfig(
				ctx,
				volumeLogger,
				r,
				volumes.ManagerContext{
					Cfg:                     vc,
					Status:                  volumeStatus.TypedSpec(),
					ParentStatus:            volumeParentStatus,
					ParentFinalizer:         parentFinalizer,
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
					TPMLocker:         hardware.LockPCRStatus(r, constants.UKIPCR, vc.Metadata().ID()),
					ShouldCloseVolume: shouldCloseVolume,
				},
			); err != nil {
				volumeStatus.TypedSpec().PreFailPhase = volumeStatus.TypedSpec().Phase
				volumeStatus.TypedSpec().Phase = block.VolumePhaseFailed
				volumeStatus.TypedSpec().ErrorMessage = err.Error()

				if xerrors.TagIs[volumes.Retryable](err) {
					shouldRetry = true
				}
			} else {
				volumeStatus.TypedSpec().ErrorMessage = ""
				volumeStatus.TypedSpec().PreFailPhase = block.VolumePhase(0)
			}

			if volumeStatus.TypedSpec().Phase != block.VolumePhaseReady {
				fullyProvisionedWave = vc.TypedSpec().Provisioning.Wave - 1
			}

			if prevPhase != volumeStatus.TypedSpec().Phase || err != nil {
				suppressVolumeLogs := slices.Contains(
					[]block.VolumeType{
						block.VolumeTypeDirectory,
						block.VolumeTypeOverlay,
						block.VolumeTypeSymlink,
					},
					volumeStatus.TypedSpec().Type,
				)

				if !suppressVolumeLogs {
					fields := []zap.Field{
						zap.String("phase", fmt.Sprintf("%s -> %s", prevPhase, volumeStatus.TypedSpec().Phase)),
						zap.Error(err),
					}

					if volumeStatus.TypedSpec().Location != "" {
						fields = append(fields, zap.String("location", volumeStatus.TypedSpec().Location))
					}

					if volumeStatus.TypedSpec().MountLocation != "" && volumeStatus.TypedSpec().MountLocation != volumeStatus.TypedSpec().Location {
						fields = append(fields, zap.String("mountLocation", volumeStatus.TypedSpec().MountLocation))
					}

					if volumeStatus.TypedSpec().ParentLocation != "" {
						fields = append(fields, zap.String("parentLocation", volumeStatus.TypedSpec().ParentLocation))
					}

					if len(volumeStatus.TypedSpec().EncryptionFailedSyncs) > 0 {
						fields = append(fields, zap.Strings("encryptionFailedSyncs", volumeStatus.TypedSpec().EncryptionFailedSyncs))
					}

					volumeLogger.Info("volume status", fields...)
				}
			}

			// when closing, ignore META volume, we want it to stay longer, so no problem if is not closed yet
			allClosed = allClosed && (volumeStatus.TypedSpec().Phase == block.VolumePhaseClosed || vc.Metadata().ID() == constants.MetaPartitionLabel)

			if shouldCloseVolume && volumeStatus.TypedSpec().Phase == block.VolumePhaseClosed {
				if volumeParentStatus != nil {
					if volumeParentStatus.Metadata().Finalizers().Has(parentFinalizer) {
						if err = r.RemoveFinalizer(ctx, volumeParentStatus.Metadata(), parentFinalizer); err != nil {
							return fmt.Errorf("error removing finalizer from parent volume configuration: %w", err)
						}
					}
				}

				// we can destroy the volume status now
				if err = r.Destroy(ctx, volumeStatus.Metadata()); err != nil {
					return fmt.Errorf("error destroying volume status: %w", err)
				}

				delete(volumeStatuses, volumeStatus.Metadata().ID())
			}
		}

		// update statuses
		for id, newVs := range volumeStatuses {
			if err = safe.WriterModify(ctx, r, block.NewVolumeStatus(block.NamespaceName, id), func(vs *block.VolumeStatus) error {
				vs.Metadata().Labels().Do(func(temp kvutils.TempKV) {
					for k, v := range newVs.Metadata().Labels().Raw() {
						temp.Set(k, v)
					}
				})

				*vs.TypedSpec() = *newVs.TypedSpec()

				return nil
			}, controller.WithExpectedPhaseAny()); err != nil {
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

func (ctrl *VolumeManagerController) progressVolumeConfig(ctx context.Context, logger *zap.Logger, r controller.Runtime, volumeContext volumes.ManagerContext) error {
	if !volumeContext.ShouldCloseVolume {
		if volumeContext.Cfg.TypedSpec().ParentID != "" {
			if volumeContext.ParentStatus == nil {
				// not ready yet
				return nil
			}

			if !volumeContext.ParentStatus.Metadata().Finalizers().Has(volumeContext.ParentFinalizer) {
				if err := r.AddFinalizer(ctx, volumeContext.ParentStatus.Metadata(), volumeContext.ParentFinalizer); err != nil {
					return fmt.Errorf("error adding finalizer to parent volume configuration: %w", err)
				}
			}
		}
	}

	return ctrl.processVolumeConfig(ctx, logger, volumeContext)
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

	for {
		if !volumeContext.ShouldCloseVolume {
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
