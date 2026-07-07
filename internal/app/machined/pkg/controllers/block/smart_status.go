// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/smartprobe"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// SMARTStatusController collects SMART health information for disks.
type SMARTStatusController struct {
	V1Alpha1Mode machineruntime.Mode

	// Prober is the SMART prober to use; if nil, a real smart.go-backed prober is used.
	// It is overridable for testing.
	Prober SMARTProber
}

// SMARTProber probes a disk for SMART data.
type SMARTProber interface {
	// Probe reads SMART data for the disk at devPath.
	//
	// When rotational is true, the disk power mode is checked first and, if the disk
	// is in standby, standby is returned true and the SMART data is not read (so the
	// disk is not spun up).
	Probe(devPath string, rotational bool) (spec block.SMARTStatusSpec, standby bool, err error)
}

// Name implements controller.Controller interface.
func (ctrl *SMARTStatusController) Name() string {
	return "block.SMARTStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SMARTStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DiskType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SMARTStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.SMARTStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SMARTStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// SMART is not available in container mode.
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	prober := ctrl.Prober
	if prober == nil {
		prober = smartprobe.Prober{}
	}

	interval := constants.DefaultDiskSMARTInterval

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching machine configuration: %w", err)
		}

		enabled := false
		newInterval := constants.DefaultDiskSMARTInterval

		if cfg != nil {
			// the presence of the DiskSMARTConfig document is what enables SMART collection.
			if smartCfg := cfg.Config().DiskSMARTConfig(); smartCfg != nil {
				enabled = true
				newInterval = smartCfg.Interval()
			}
		}

		if newInterval != interval {
			interval = newInterval
			ticker.Reset(interval)
		}

		r.StartTrackingOutputs()

		if enabled {
			if err := ctrl.probeDisks(ctx, r, logger, prober); err != nil {
				return err
			}
		}

		if err := safe.CleanupOutputs[*block.SMARTStatus](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up SMART statuses: %w", err)
		}
	}
}

func (ctrl *SMARTStatusController) probeDisks(ctx context.Context, r controller.Runtime, logger *zap.Logger, prober SMARTProber) error {
	disks, err := safe.ReaderListAll[*block.Disk](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list disks: %w", err)
	}

	for disk := range disks.All() {
		diskSpec := disk.TypedSpec()

		// skip CD-ROMs and disks without a real device path.
		if diskSpec.CDROM || diskSpec.DevPath == "" {
			continue
		}

		spec, standby, err := prober.Probe(diskSpec.DevPath, diskSpec.Rotational)
		if err != nil {
			// many virtual/USB disks don't support SMART: don't fail the controller,
			// just skip them (the SMARTStatus, if any, is reaped by CleanupOutputs).
			logger.Debug("failed to probe disk for SMART data", zap.String("disk", disk.Metadata().ID()), zap.Error(err))

			continue
		}

		if err := safe.WriterModify(ctx, r, block.NewSMARTStatus(block.NamespaceName, disk.Metadata().ID()),
			func(s *block.SMARTStatus) error {
				if standby {
					// don't overwrite previously collected SMART data with an empty
					// spec: only refresh the power state, keeping prior values.
					if s.TypedSpec().DevPath == "" {
						s.TypedSpec().DevPath = spec.DevPath
						s.TypedSpec().DevType = spec.DevType
						s.TypedSpec().Healthy = true
					}

					s.TypedSpec().PowerState = spec.PowerState
					s.TypedSpec().Message = "skipped: disk in standby"

					return nil
				}

				*s.TypedSpec() = spec

				return nil
			}); err != nil {
			return fmt.Errorf("failed to update SMART status for %q: %w", disk.Metadata().ID(), err)
		}
	}

	return nil
}
