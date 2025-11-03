// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	blockadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton/blockautomaton"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/xfs"
)

// PlatformConfigLoadController loads cached platform network config from STATE.
type PlatformConfigLoadController struct {
	stateMachine blockautomaton.VolumeMounterAutomaton
}

// Name implements controller.Controller interface.
func (ctrl *PlatformConfigLoadController) Name() string {
	return "network.PlatformConfigLoadController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PlatformConfigLoadController) Inputs() []controller.Input {
	return []controller.Input{
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
func (ctrl *PlatformConfigLoadController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.PlatformConfigType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *PlatformConfigLoadController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if ctrl.stateMachine == nil {
			ctrl.stateMachine = blockautomaton.NewVolumeMounter(
				ctrl.Name(),
				constants.StatePartitionLabel,
				ctrl.load(),
				blockautomaton.WithReadOnly(true),
				blockautomaton.WithDetached(true),
			)
		}

		if err := ctrl.stateMachine.Run(ctx, r, logger,
			automaton.WithAfterFunc(func() error {
				ctrl.stateMachine = nil

				return nil
			}),
		); err != nil {
			return fmt.Errorf("error running volume mounter machine: %w", err)
		}

		if ctrl.stateMachine == nil {
			// we read only once, so once read, we should stop
			return nil
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *PlatformConfigLoadController) load() func(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus,
) error {
	return func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus) error {
		return blockadapter.VolumeMountStatus(mountStatus).WithRoot(logger, func(root xfs.Root) error {
			cachedNetworkConfig, err := ctrl.loadConfig(root, constants.PlatformNetworkConfigFilename)
			if err != nil {
				logger.Warn("ignored failure loading cached platform network config", zap.Error(err))
			} else if cachedNetworkConfig != nil {
				logger.Debug("loaded cached platform network config")
			}

			if cachedNetworkConfig != nil {
				if err := safe.WriterModify(ctx, r,
					network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigCachedID),
					func(out *network.PlatformConfig) error {
						*out.TypedSpec() = *cachedNetworkConfig

						return nil
					},
				); err != nil {
					return fmt.Errorf("error modifying cached platform network config: %w", err)
				}
			}

			return nil
		})
	}
}

func (ctrl *PlatformConfigLoadController) loadConfig(root xfs.Root, path string) (*network.PlatformConfigSpec, error) {
	marshaled, err := xfs.ReadFile(root, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	var networkConfig network.PlatformConfigSpec

	if err = yaml.Unmarshal(marshaled, &networkConfig); err != nil {
		return nil, fmt.Errorf("error unmarshaling network config: %w", err)
	}

	return &networkConfig, nil
}
