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

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/smart"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

const defaultHealthInterval = 5 * time.Minute

// DiskHealthStatusController collects disk health data and publishes DiskHealthStatus resources.
type DiskHealthStatusController struct {
	V1Alpha1Mode machineruntime.Mode
	Collector    smart.Collector
}

// Name implements controller.Controller interface.
func (ctrl *DiskHealthStatusController) Name() string {
	return "block.DiskHealthStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DiskHealthStatusController) Inputs() []controller.Input {
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
func (ctrl *DiskHealthStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.DiskHealthStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *DiskHealthStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	if ctrl.Collector == nil {
		ctrl.Collector = &smart.RealCollector{}
	}

	ticker := time.NewTicker(defaultHealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		enabled, interval := ctrl.readConfig(ctx, r, logger)

		ticker.Reset(interval)

		r.StartTrackingOutputs()

		if enabled {
			if err := ctrl.collectHealth(ctx, r, logger); err != nil {
				return fmt.Errorf("failed to collect disk health: %w", err)
			}
		}

		if err := safe.CleanupOutputs[*block.DiskHealthStatus](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

func (ctrl *DiskHealthStatusController) readConfig(ctx context.Context, r controller.Reader, logger *zap.Logger) (bool, time.Duration) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, defaultHealthInterval
		}

		logger.Warn("failed to read config, using defaults", zap.Error(err))

		return true, defaultHealthInterval
	}

	var dhCfg configconfig.DiskHealthMonitoringConfig
	if cfg != nil {
		dhCfg = cfg.Config().DiskHealthMonitoringConfig()
	}

	if dhCfg == nil {
		return true, defaultHealthInterval
	}

	return dhCfg.DiskHealthMonitoringEnabled(), dhCfg.DiskHealthMonitoringInterval()
}

func (ctrl *DiskHealthStatusController) collectHealth(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	disks, err := safe.ReaderListAll[*block.Disk](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list disks: %w", err)
	}

	for disk := range disks.All() {
		diskID := disk.Metadata().ID()
		devPath := disk.TypedSpec().DevPath

		result := ctrl.Collector.Collect(devPath)

		if result.Error != "" {
			logger.Debug("disk health collection issue",
				zap.String("disk", diskID),
				zap.String("error", result.Error),
			)
		}

		if err := safe.WriterModify(ctx, r, block.NewDiskHealthStatus(block.NamespaceName, diskID),
			func(dhs *block.DiskHealthStatus) error {
				spec := dhs.TypedSpec()
				spec.DiskID = diskID
				spec.Device = devPath
				spec.HealthSource = result.Source
				spec.Status = result.Status
				spec.TemperatureCelsius = result.TemperatureCelsius
				spec.PowerOnHours = result.PowerOnHours
				spec.PowerCycles = result.PowerCycles
				spec.LastChecked = time.Now()
				spec.Error = result.Error
				spec.Details = block.DiskHealthDetails{
					NVMe: result.NVMe,
					ATA:  result.ATA,
				}

				return nil
			},
		); err != nil {
			return fmt.Errorf("failed to modify disk health status for %q: %w", diskID, err)
		}
	}

	return nil
}
