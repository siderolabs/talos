// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"slices"
	"time"

	smart "github.com/anatol/smart.go"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

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
		prober = smartGoProber{}
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

		enabled := true
		newInterval := constants.DefaultDiskSMARTInterval

		if cfg != nil {
			if smartCfg := cfg.Config().DiskSMARTConfig(); smartCfg != nil {
				enabled = smartCfg.Enabled()
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
						s.TypedSpec().DeviceType = spec.DeviceType
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

// smartGoProber is the default SMARTProber backed by the smart.go library.
type smartGoProber struct{}

func (smartGoProber) Probe(devPath string, rotational bool) (block.SMARTStatusSpec, bool, error) {
	dev, err := smart.Open(devPath)
	if err != nil {
		return block.SMARTStatusSpec{}, false, err
	}

	defer dev.Close() //nolint:errcheck

	spec := block.SMARTStatusSpec{
		DevPath:    devPath,
		DeviceType: dev.Type(),
		Healthy:    true,
	}

	switch d := dev.(type) {
	case *smart.SataDevice:
		// avoid spinning up a standby disk just to read SMART.
		if rotational {
			if mode, err := d.CheckPowerMode(); err == nil {
				spec.PowerState = powerModeString(mode)

				if mode == smart.PowerModeStandby {
					return spec, true, nil
				}
			}
		}

		if err := fillSATA(&spec, d); err != nil {
			return block.SMARTStatusSpec{}, false, err
		}
	case *smart.NVMeDevice:
		if err := fillNVMe(&spec, d); err != nil {
			return block.SMARTStatusSpec{}, false, err
		}
	default:
		fillGeneric(&spec, dev)
	}

	return spec, false, nil
}

func fillGeneric(spec *block.SMARTStatusSpec, dev smart.Device) {
	attrs, err := dev.ReadGenericAttributes()
	if err != nil {
		return
	}

	spec.Temperature = uint32(attrs.Temperature)
	spec.PowerOnHours = attrs.PowerOnHours
	spec.PowerCycles = attrs.PowerCycles
}

func fillNVMe(spec *block.SMARTStatusSpec, d *smart.NVMeDevice) error {
	sm, err := d.ReadSMART()
	if err != nil {
		return err
	}

	spec.CriticalWarning = uint32(sm.CritWarning)
	spec.Healthy = sm.CritWarning == 0
	spec.AvailableSpare = uint32(sm.AvailSpare)
	spec.PercentUsed = uint32(sm.PercentUsed)
	spec.PowerOnHours = sm.PowerOnHours.Val[0]
	spec.PowerCycles = sm.PowerCycles.Val[0]
	spec.MediaErrors = sm.MediaErrors.Val[0]

	// Composite temperature is reported in Kelvin.
	if sm.Temperature >= 273 {
		spec.Temperature = uint32(sm.Temperature) - 273
	}

	return nil
}

func fillSATA(spec *block.SMARTStatusSpec, d *smart.SataDevice) error {
	page, err := d.ReadSMARTData()
	if err != nil {
		return err
	}

	// thresholds are best-effort; without them we can't compute per-attribute failure.
	thresholds, _ := d.ReadSMARTThresholds() //nolint:errcheck

	healthy := true

	for id, attr := range page.Attrs {
		sa := block.SMARTAttribute{
			ID:       uint32(attr.Id),
			Name:     attr.Name,
			Current:  uint32(attr.Current),
			Worst:    uint32(attr.Worst),
			RawValue: attr.ValueRaw,
		}

		if thresholds != nil {
			if th, ok := thresholds.Thresholds[id]; ok {
				sa.Threshold = uint32(th)

				// an attribute is failing when its normalized current value drops to or
				// below the threshold; a failing pre-failure attribute marks the disk unhealthy.
				if th > 0 && attr.Current <= th {
					sa.Failing = true

					if attr.Flags&smart.AtaAttributeFlagPrefailure != 0 {
						healthy = false
					}
				}
			}
		}

		spec.Attributes = append(spec.Attributes, sa)
	}

	// sort attributes by ID for a stable resource representation (map order is random).
	slices.SortFunc(spec.Attributes, func(a, b block.SMARTAttribute) int {
		return int(a.ID) - int(b.ID)
	})

	spec.Healthy = healthy

	fillGeneric(spec, d)

	return nil
}

func powerModeString(mode byte) string {
	switch mode {
	case smart.PowerModeStandby:
		return "standby"
	case smart.PowerModeIdle:
		return "idle"
	default:
		return "active"
	}
}
