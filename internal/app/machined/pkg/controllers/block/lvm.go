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

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
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
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
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
		ctrl.seenVolumes = map[string]struct{}{}
	}

	if ctrl.activatedVGs == nil {
		ctrl.activatedVGs = map[string]struct{}{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		udevdService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "udevd")
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get udevd service: %w", err)
		}

		if udevdService == nil {
			logger.Debug("udevd service not registered yet")

			continue
		}

		if !(udevdService.TypedSpec().Running && udevdService.TypedSpec().Healthy) {
			logger.Debug("waiting for udevd service to be running and healthy")

			continue
		}

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		var multiErr error

		for dv := range discoveredVolumes.All() {
			if dv.TypedSpec().Name != "lvm2-pv" {
				// if the volume is not an LVM volume the moment we saw it, we can skip it
				// we need to activate the volumes only on reboot, not when they are first formatted
				ctrl.seenVolumes[dv.Metadata().ID()] = struct{}{}

				continue
			}

			if _, ok := ctrl.seenVolumes[dv.Metadata().ID()]; ok {
				continue
			}

			logger.Debug("checking device for LVM volume activation", zap.String("device", dv.TypedSpec().DevPath))

			vgName, err := ctrl.checkVGNeedsActivation(ctx, dv.TypedSpec().DevPath)
			if err != nil {
				multiErr = multierror.Append(multiErr, err)

				continue
			}

			if vgName == "" {
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
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to activate LVM volume %s: %w", vgName, err))
			} else {
				ctrl.activatedVGs[vgName] = struct{}{}
			}
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

	// parse the key-value pairs from the udev output
	for _, line := range strings.Split(stdOut, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		value = strings.Trim(value, "'\"")

		if key == "LVM_VG_NAME_COMPLETE" {
			return value, nil
		}
	}

	return "", nil
}
