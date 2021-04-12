// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/ntp"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/time"
)

// SyncController manages v1alpha1.TimeSync based on configuration and NTP sync process.
type SyncController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
	NewNTPSyncer NewNTPSyncerFunc
}

// Name implements controller.Controller interface.
func (ctrl *SyncController) Name() string {
	return "time.SyncController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SyncController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
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
type NewNTPSyncerFunc func(*log.Logger, []string) NTPSyncer

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *SyncController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if ctrl.NewNTPSyncer == nil {
		ctrl.NewNTPSyncer = func(logger *log.Logger, timeServers []string) NTPSyncer {
			return ntp.NewSyncer(logger, timeServers)
		}
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
	)

	defer func() {
		if syncer != nil {
			syncCtxCancel()

			syncWg.Wait()
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
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		syncDisabled := false

		if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
			syncDisabled = true
		}

		if cfg != nil && cfg.(*config.MachineConfig).Config().Machine().Time().Disabled() {
			syncDisabled = true
		}

		timeServers := []string{constants.DefaultNTPServer}
		if cfg != nil {
			timeServers = cfg.(*config.MachineConfig).Config().Machine().Time().Servers()
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

			syncCtx, syncCtxCancel = context.WithCancel(ctx) //nolint:govet

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

		if err = r.Modify(ctx, time.NewStatus(), func(r resource.Resource) error {
			r.(*time.Status).SetStatus(time.StatusSpec{
				Epoch:        epoch,
				Synced:       timeSynced,
				SyncDisabled: syncDisabled,
			})

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err) //nolint:govet
		}
	}
}
