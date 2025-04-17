// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// PCRStatusController manages TPM PCR extension.
type PCRStatusController struct {
	V1Alpha1Mode runtimetalos.Mode

	numberOfExtensions int
}

// Name implements controller.Controller interface.
func (ctrl *PCRStatusController) Name() string {
	return "hardware.PCRStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PCRStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PCRStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.PCRStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *PCRStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// PCR status doesn't make sense inside a container, so skip the controller
	if ctrl.V1Alpha1Mode == runtimetalos.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		switch ctrl.numberOfExtensions {
		case 0:
			// extend the PCR for the first time
			// this unlock initial PCR extension
			if err := tpm2.PCRExtend(constants.UKIPCR, []byte(secureboot.EnterMachined)); err != nil {
				return fmt.Errorf("error performing initial PCR extension: %w", err)
			}

			if err := r.Create(ctx, hardware.NewPCCRStatus(constants.UKIPCR)); err != nil {
				return fmt.Errorf("error creating PCRStatus resource: %w", err)
			}

			logger.Info("TPM is ready for disk encryption operations (if available)")

			ctrl.numberOfExtensions++
		case 1:
			// as long as Volumes were provisioned, we extend the PCR once again locking further access to the TPM
			volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
			if err != nil {
				return fmt.Errorf("error listing volume statuses: %w", err)
			}

			volumesReady := map[string]struct{}{}
			volumesPending := map[string]struct{}{}

			for volumeStatus := range volumeStatuses.All() {
				switch volumeStatus.TypedSpec().Type {
				case block.VolumeTypeDisk, block.VolumeTypePartition:
					// can be encrypted
				case block.VolumeTypeDirectory, block.VolumeTypeOverlay, block.VolumeTypeSymlink, block.VolumeTypeTmpfs:
					// skip it, not encryptable
					continue
				}

				switch volumeStatus.TypedSpec().Phase {
				case block.VolumePhaseMissing:
					// skip it, missing
				case block.VolumePhaseReady:
					volumesReady[volumeStatus.Metadata().ID()] = struct{}{}
				case block.VolumePhaseClosed:
					// skip it, closed
				case block.VolumePhaseLocated, block.VolumePhaseWaiting, block.VolumePhaseFailed,
					block.VolumePhaseProvisioned, block.VolumePhasePrepared:
					volumesPending[volumeStatus.Metadata().ID()] = struct{}{}
				}
			}

			notReady := false

			for _, requiredVolumeID := range []string{constants.StatePartitionLabel, constants.EphemeralPartitionLabel} {
				if _, ready := volumesReady[requiredVolumeID]; !ready {
					logger.Debug("skipping PCR extension, volume not ready", zap.String("volume", requiredVolumeID))

					notReady = true

					break
				}
			}

			if notReady {
				continue
			}

			if len(volumesPending) > 0 {
				pendingVolumes := slices.Sorted(maps.Keys(volumesPending))

				logger.Debug("skipping PCR extension, volumes not ready", zap.Strings("volumes", pendingVolumes))

				continue
			}

			// ready to extend
			readyToDestroy, err := r.Teardown(ctx, hardware.NewPCCRStatus(constants.UKIPCR).Metadata())
			if err != nil {
				return fmt.Errorf("error tearing down PCRStatus resource: %w", err)
			}

			if !readyToDestroy {
				continue
			}

			if err = r.Destroy(ctx, hardware.NewPCCRStatus(constants.UKIPCR).Metadata()); err != nil {
				return fmt.Errorf("error destroying PCRStatus resource: %w", err)
			}

			if err := tpm2.PCRExtend(constants.UKIPCR, []byte(secureboot.StartTheWorld)); err != nil {
				return fmt.Errorf("error performing PCR extension: %w", err)
			}

			logger.Info("TPM is locked to block any disk encryption operation (if available)")

			ctrl.numberOfExtensions++
		case 2: // nothing to do, we are done
		}

		r.ResetRestartBackoff()
	}
}
