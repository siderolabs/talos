// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton/blockautomaton"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// PlatformConfigStoreController stores (caches) active platform network config in STATE.
type PlatformConfigStoreController struct {
	stateMachine                    blockautomaton.VolumeMounterAutomaton
	configToStore, lastStoredConfig *network.PlatformConfig
}

// Name implements controller.Controller interface.
func (ctrl *PlatformConfigStoreController) Name() string {
	return "network.PlatformConfigStoreController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PlatformConfigStoreController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.PlatformConfigType,
			ID:        optional.Some(network.PlatformConfigActiveID),
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PlatformConfigStoreController) Outputs() []controller.Output {
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
func (ctrl *PlatformConfigStoreController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		activeConfig, err := safe.ReaderGetByID[*network.PlatformConfig](ctx, r, network.PlatformConfigActiveID)
		if err != nil {
			if state.IsNotFoundError(err) {
				// no active network config found, wait more
				continue
			}

			return fmt.Errorf("error getting active network config: %w", err)
		}

		// if we haven't stored any config yet, or the active config has changed
		if ctrl.lastStoredConfig == nil || !activeConfig.TypedSpec().Equal(ctrl.lastStoredConfig.TypedSpec()) {
			ctrl.configToStore = activeConfig
		}

		if ctrl.stateMachine == nil && ctrl.configToStore != nil {
			ctrl.stateMachine = blockautomaton.NewVolumeMounter(
				ctrl.Name(), constants.StatePartitionLabel,
				ctrl.store(),
			)
		}

		if ctrl.stateMachine != nil {
			if err := ctrl.stateMachine.Run(ctx, r, logger,
				automaton.WithAfterFunc(func() error {
					ctrl.stateMachine = nil

					return nil
				}),
			); err != nil {
				return fmt.Errorf("error running volume mounter machine: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *PlatformConfigStoreController) store() func(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus,
) error {
	return func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus) error {
		rootPath := mountStatus.TypedSpec().Target

		if err := ctrl.storeConfig(filepath.Join(rootPath, constants.PlatformNetworkConfigFilename), ctrl.configToStore); err != nil {
			return fmt.Errorf("error saving platform network config: %w", err)
		}

		// remember last stored config
		ctrl.lastStoredConfig, ctrl.configToStore = ctrl.configToStore, nil

		logger.Debug("stored active platform network config")

		return nil
	}
}

func (ctrl *PlatformConfigStoreController) storeConfig(path string, networkConfig *network.PlatformConfig) error {
	marshaled, err := yaml.Marshal(networkConfig.TypedSpec())
	if err != nil {
		return fmt.Errorf("error marshaling network config: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		existing, err := os.ReadFile(path)
		if err == nil && bytes.Equal(marshaled, existing) {
			// existing contents are identical, skip writing to avoid no-op writes
			return nil
		}
	}

	return os.WriteFile(path, marshaled, 0o400)
}
