// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-cmd/pkg/cmd"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// LVMActivationController activates LVM volumes when they are discovered by the block.DiscoveryController.
type LVMActivationController struct {
	seenVolumes  map[string]struct{}
	activatedVGs map[string]struct{}
}

// Name implements controller.Controller interface.
func (ctrl *LVMActivationController) Name() string {
	return "block.LVMActivationController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMActivationController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      runtimeres.MountStatusType,
			ID:        optional.Some(constants.EphemeralPartitionLabel),
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
func (ctrl *LVMActivationController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *LVMActivationController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.seenVolumes == nil {
		ctrl.seenVolumes = make(map[string]struct{})
	}

	if ctrl.activatedVGs == nil {
		ctrl.activatedVGs = make(map[string]struct{})
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if _, err := safe.ReaderGetByID[*runtimeres.MountStatus](ctx, r, constants.EphemeralPartitionLabel); err != nil {
			if state.IsNotFoundError(err) {
				// wait for the mount status to be available
				continue
			}

			return fmt.Errorf("failed to get mount status: %w", err)
		}

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		var multiErr error

		for iterator := discoveredVolumes.Iterator(); iterator.Next(); {
			if _, ok := ctrl.seenVolumes[iterator.Value().Metadata().ID()]; ok {
				continue
			}

			if iterator.Value().TypedSpec().Name != "lvm2-pv" {
				ctrl.seenVolumes[iterator.Value().Metadata().ID()] = struct{}{}

				continue
			}

			logger.Info("checking device for LVM volume activation", zap.String("device", iterator.Value().TypedSpec().DevPath))

			vgName, err := ctrl.checkVGNeedsActivation(ctx, iterator.Value().TypedSpec().DevPath)
			if err != nil {
				multiErr = multierror.Append(multiErr, err)

				continue
			}

			if vgName == "" {
				ctrl.seenVolumes[iterator.Value().Metadata().ID()] = struct{}{}

				continue
			}

			if _, ok := ctrl.activatedVGs[vgName]; ok {
				continue
			}

			logger.Info("activating LVM volume", zap.String("name", vgName))

			// activate the volume group
			if _, err = cmd.RunContext(ctx,
				"/sbin/lvm",
				"vgchange",
				"-aay",
				"--autoactivation",
				"event",
				vgName,
			); err != nil {
				return fmt.Errorf("failed to activate LVM volume %s: %w", vgName, err)
			}

			ctrl.activatedVGs[vgName] = struct{}{}
		}

		if multiErr != nil {
			return multiErr
		}
	}
}

// checkVGNeedsActivation checks if the device is part of a volume group and returns the volume group name
// if it needs to be activated, otherwise it returns an empty string.
func (ctrl *LVMActivationController) checkVGNeedsActivation(ctx context.Context, devicePath string) (string, error) {
	// first we check if all associated volumes are available
	// https://man7.org/linux/man-pages/man7/lvmautoactivation.7.html
	stdOut, err := cmd.RunContext(ctx,
		"/sbin/lvm",
		"pvscan",
		"--cache",
		"--verbose",
		"--listvg",
		"--checkcomplete",
		"--vgonline",
		"--autoactivation",
		"event",
		"--udevoutput",
		devicePath,
	)
	if err != nil {
		return "", fmt.Errorf("failed to check if LVM volume backed by device %s needs activation: %w", devicePath, err)
	}

	if strings.HasPrefix(stdOut, "LVM_VG_NAME_INCOMPLETE") {
		return "", nil
	}

	vgName := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(stdOut, "\n"), "LVM_VG_NAME_COMPLETE='"), "'")

	return vgName, nil
}
