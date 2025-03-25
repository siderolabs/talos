// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"context"
	"fmt"
	"net"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	kubespanadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/kubespan"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton/blockautomaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// IdentityController watches KubeSpan configuration, updates KubeSpan Identity.
type IdentityController struct {
	stateMachine blockautomaton.VolumeMounterAutomaton
}

// Name implements controller.Controller interface.
func (ctrl *IdentityController) Name() string {
	return "kubespan.IdentityController"
}

// Inputs implements controller.Controller interface.
func (ctrl *IdentityController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      kubespan.ConfigType,
			ID:        optional.Some(kubespan.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HardwareAddrType,
			ID:        optional.Some(network.FirstHardwareAddr),
			Kind:      controller.InputWeak,
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
func (ctrl *IdentityController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: kubespan.IdentityType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *IdentityController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*kubespan.Config](ctx, r, kubespan.ConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting kubespan configuration: %w", err)
		}

		firstMAC, err := safe.ReaderGetByID[*network.HardwareAddr](ctx, r, network.FirstHardwareAddr)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting first MAC address: %w", err)
		}

		_, err = safe.ReaderGetByID[*kubespan.Identity](ctx, r, kubespan.LocalIdentity)
		alreadyHasIdentity := err == nil

		if cfg != nil && firstMAC != nil && cfg.TypedSpec().Enabled {
			if ctrl.stateMachine == nil && !alreadyHasIdentity {
				ctrl.stateMachine = blockautomaton.NewVolumeMounter(ctrl.Name(), constants.StatePartitionLabel, ctrl.establishIdentity(cfg, firstMAC))
			}
		} else if alreadyHasIdentity {
			if err = r.Destroy(ctx, kubespan.NewIdentity(kubespan.NamespaceName, kubespan.LocalIdentity).Metadata()); err != nil {
				return fmt.Errorf("error cleaning up identity: %w", err)
			}
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

func (ctrl *IdentityController) establishIdentity(
	cfg *kubespan.Config, firstMAC *network.HardwareAddr,
) func(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus,
) error {
	return func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus) error {
		rootPath := mountStatus.TypedSpec().Target

		var localIdentity kubespan.IdentitySpec

		if err := controllers.LoadOrNewFromFile(filepath.Join(rootPath, constants.KubeSpanIdentityFilename), &localIdentity, func(v *kubespan.IdentitySpec) error {
			return kubespanadapter.IdentitySpec(v).GenerateKey()
		}); err != nil {
			return fmt.Errorf("error caching kubespan identity: %w", err)
		}

		kubespanCfg := cfg.TypedSpec()
		mac := firstMAC.TypedSpec()

		if err := kubespanadapter.IdentitySpec(&localIdentity).UpdateAddress(kubespanCfg.ClusterID, net.HardwareAddr(mac.HardwareAddr)); err != nil {
			return fmt.Errorf("error updating KubeSpan address: %w", err)
		}

		return safe.WriterModify(ctx, r, kubespan.NewIdentity(kubespan.NamespaceName, kubespan.LocalIdentity), func(res *kubespan.Identity) error {
			*res.TypedSpec() = localIdentity

			return nil
		})
	}
}
