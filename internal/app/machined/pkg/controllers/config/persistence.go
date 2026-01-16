// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	blockadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton/blockautomaton"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/xfs"
)

// PersistenceController ensures that the machine configuration is persisted in STATE partition.
type PersistenceController struct {
	lastPersistedVersion resource.Version
	configToPersist      *config.MachineConfig
	stateMachine         blockautomaton.VolumeMounterAutomaton
}

// Name implements controller.Controller interface.
func (ctrl *PersistenceController) Name() string {
	return "config.PersistenceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PersistenceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.PersistentID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: resources.InMemoryNamespace,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
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
func (ctrl *PersistenceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *PersistenceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
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

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.PersistentID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting config: %w", err)
		}

		if cfg != nil && !ctrl.lastPersistedVersion.Equal(cfg.Metadata().Version()) {
			// if the version is newer than the last persisted version
			ctrl.configToPersist = cfg
		}

		if ctrl.stateMachine == nil && ctrl.configToPersist != nil {
			ctrl.stateMachine = blockautomaton.NewVolumeMounter(
				ctrl.Name(),
				constants.StatePartitionLabel,
				ctrl.persistMachineConfig,
				blockautomaton.WithDetached(true),
			)
		}

		if ctrl.stateMachine != nil {
			err := ctrl.stateMachine.Run(ctx, r, logger,
				automaton.WithAfterFunc(func() error {
					ctrl.stateMachine = nil

					r.QueueReconcile()

					return nil
				}),
			)
			if err != nil {
				return fmt.Errorf("error running state machine: %w", err)
			}
		}

		if volumeLifecycle.Metadata().Phase() == resource.PhaseTearingDown {
			if ctrl.configToPersist == nil {
				if err = r.RemoveFinalizer(ctx, volumeLifecycle.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *PersistenceController) persistMachineConfig(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus) error {
	return blockadapter.VolumeMountStatus(mountStatus).WithRoot(logger, func(root xfs.Root) error {
		tempName := constants.ConfigFilename + "-tmp"

		configContents, err := ctrl.configToPersist.Provider().Bytes()
		if err != nil {
			return fmt.Errorf("error getting config bytes: %w", err)
		}

		if err = xfs.WriteFile(root, tempName, configContents, 0o600); err != nil {
			return fmt.Errorf("error writing config to file: %w", err)
		}

		if err = xfs.Rename(root, tempName, constants.ConfigFilename); err != nil {
			return fmt.Errorf("error renaming config file: %w", err)
		}

		logger.Info("machine configuration persisted to STATE")

		ctrl.lastPersistedVersion = ctrl.configToPersist.Metadata().Version()
		ctrl.configToPersist = nil

		return nil
	})
}
