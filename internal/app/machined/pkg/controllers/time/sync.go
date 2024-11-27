// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"context"
	"fmt"
	"sync"
	stdtime "time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/ntp"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/time"
)

// SyncController manages v1alpha1.TimeSync based on configuration and NTP sync process.
type SyncController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
	NewNTPSyncer NewNTPSyncerFunc

	bootTime stdtime.Time
}

// Name implements controller.Controller interface.
func (ctrl *SyncController) Name() string {
	return "time.SyncController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SyncController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *SyncController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: time.StatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// NTPSyncer interface is implemented by ntp.Syncer, interface for mocking.
type NTPSyncer interface {
	Run(ctx context.Context)
	Synced() <-chan struct{}
	EpochChange() <-chan struct{}
	SetTimeServers([]string)
}

// NewNTPSyncerFunc function allows to replace ntp.Syncer with the mock.
type NewNTPSyncerFunc func(*zap.Logger, []string) NTPSyncer

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *SyncController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.bootTime.IsZero() {
		ctrl.bootTime = stdtime.Now()
	}

	if ctrl.NewNTPSyncer == nil {
		ctrl.NewNTPSyncer = func(logger *zap.Logger, timeServers []string) NTPSyncer {
			return ntp.NewSyncer(logger, timeServers)
		}
	}

	// wait for udevd to be healthy, which implies that all RTC devices
	if err := runtime.WaitForDevicesReady(ctx, r,
		[]controller.Input{
			{
				Namespace: network.NamespaceName,
				Type:      network.TimeServerStatusType,
				ID:        optional.Some(network.TimeServerID),
				Kind:      controller.InputWeak,
			},
			{
				Namespace: config.NamespaceName,
				Type:      config.MachineConfigType,
				ID:        optional.Some(config.V1Alpha1ID),
			},
		},
	); err != nil {
		return err
	}

	var (
		syncCtx       context.Context
		syncCtxCancel context.CancelFunc
		syncWg        sync.WaitGroup

		syncCh  <-chan struct{}
		epochCh <-chan struct{}
		syncer  NTPSyncer

		timeSynced bool
		epoch      int

		timeSyncTimeoutTimer *stdtime.Timer
		timeSyncTimeoutCh    <-chan stdtime.Time
	)

	defer func() {
		if syncer != nil {
			syncCtxCancel()

			syncWg.Wait()
		}

		if timeSyncTimeoutTimer != nil {
			timeSyncTimeoutTimer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-syncCh:
			syncCh = nil
			timeSynced = true
		case <-epochCh:
			epoch++
		case <-timeSyncTimeoutCh:
			timeSynced = true
			timeSyncTimeoutTimer = nil
		}

		timeServersStatus, err := safe.ReaderGet[*network.TimeServerStatus](
			ctx,
			r,
			resource.NewMetadata(network.NamespaceName, network.TimeServerStatusType, network.TimeServerID, resource.VersionUndefined),
		)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting time server status: %w", err)
			}

			// time server list is not ready yet, wait for the next reconcile
			continue
		}

		timeServers := timeServersStatus.TypedSpec().NTPServers

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		var syncTimeout stdtime.Duration

		syncDisabled := false

		if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
			syncDisabled = true
		}

		if cfg != nil && cfg.Config().Machine() != nil {
			if cfg.Config().Machine().Time().Disabled() {
				syncDisabled = true
			}

			syncTimeout = cfg.Config().Machine().Time().BootTimeout()
		}

		if !timeSynced {
			sinceBoot := stdtime.Since(ctrl.bootTime)

			switch {
			case syncTimeout == 0:
				// disable sync timeout
				if timeSyncTimeoutTimer != nil {
					timeSyncTimeoutTimer.Stop()
				}

				timeSyncTimeoutCh = nil
			case sinceBoot > syncTimeout:
				// over sync timeout already, so in sync
				timeSynced = true
			default:
				// make sure timer fires in whatever time is left till the timeout
				if timeSyncTimeoutTimer == nil || !timeSyncTimeoutTimer.Reset(syncTimeout-sinceBoot) {
					timeSyncTimeoutTimer = stdtime.NewTimer(syncTimeout - sinceBoot)
					timeSyncTimeoutCh = timeSyncTimeoutTimer.C
				}
			}
		}

		switch {
		case syncDisabled && syncer != nil:
			// stop syncing
			syncCtxCancel()

			syncWg.Wait()

			syncer = nil
			syncCh = nil
			epochCh = nil
		case !syncDisabled && syncer == nil:
			// start syncing
			syncer = ctrl.NewNTPSyncer(logger, timeServers)
			syncCh = syncer.Synced()
			epochCh = syncer.EpochChange()

			timeSynced = false

			syncCtx, syncCtxCancel = context.WithCancel(ctx) //nolint:govet,fatcontext

			syncWg.Add(1)

			go func() {
				defer syncWg.Done()

				syncer.Run(syncCtx)
			}()
		}

		if syncer != nil {
			syncer.SetTimeServers(timeServers)
		}

		if syncDisabled {
			timeSynced = true
		}

		if err = safe.WriterModify(ctx, r, time.NewStatus(), func(r *time.Status) error {
			*r.TypedSpec() = time.StatusSpec{
				Epoch:        epoch,
				Synced:       timeSynced,
				SyncDisabled: syncDisabled,
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err) //nolint:govet
		}

		r.ResetRestartBackoff()
	}
}
