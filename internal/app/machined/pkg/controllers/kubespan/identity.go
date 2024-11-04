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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	kubespanadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/kubespan"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// IdentityController watches KubeSpan configuration, updates KubeSpan Identity.
type IdentityController struct {
	StatePath string
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
			Namespace: v1alpha1.NamespaceName,
			Type:      runtimeres.MountStatusType,
			ID:        optional.Some(constants.StatePartitionLabel),
			Kind:      controller.InputWeak,
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
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *IdentityController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.StatePath == "" {
		ctrl.StatePath = constants.StateMountPoint
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if _, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, runtimeres.MountStatusType, constants.StatePartitionLabel, resource.VersionUndefined)); err != nil {
				if state.IsNotFoundError(err) {
					// wait for STATE to be mounted
					continue
				}

				return fmt.Errorf("error reading mount status: %w", err)
			}

			cfg, err := safe.ReaderGet[*kubespan.Config](ctx, r, resource.NewMetadata(config.NamespaceName, kubespan.ConfigType, kubespan.ConfigID, resource.VersionUndefined))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting kubespan configuration: %w", err)
			}

			firstMAC, err := safe.ReaderGet[*network.HardwareAddr](ctx, r, resource.NewMetadata(network.NamespaceName, network.HardwareAddrType, network.FirstHardwareAddr, resource.VersionUndefined))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting first MAC address: %w", err)
			}

			touchedIDs := make(map[resource.ID]struct{})

			if cfg != nil && firstMAC != nil && cfg.TypedSpec().Enabled {
				var localIdentity kubespan.IdentitySpec

				if err = controllers.LoadOrNewFromFile(filepath.Join(ctrl.StatePath, constants.KubeSpanIdentityFilename), &localIdentity, func(v any) error {
					return kubespanadapter.IdentitySpec(v.(*kubespan.IdentitySpec)).GenerateKey()
				}); err != nil {
					return fmt.Errorf("error caching kubespan identity: %w", err)
				}

				kubespanCfg := cfg.TypedSpec()
				mac := firstMAC.TypedSpec()

				if err = kubespanadapter.IdentitySpec(&localIdentity).UpdateAddress(kubespanCfg.ClusterID, net.HardwareAddr(mac.HardwareAddr)); err != nil {
					return fmt.Errorf("error updating KubeSpan address: %w", err)
				}

				if err = safe.WriterModify(ctx, r, kubespan.NewIdentity(kubespan.NamespaceName, kubespan.LocalIdentity), func(res *kubespan.Identity) error {
					*res.TypedSpec() = localIdentity

					return nil
				}); err != nil {
					return err
				}

				touchedIDs[kubespan.LocalIdentity] = struct{}{}
			}

			// list keys for cleanup
			list, err := r.List(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.IdentityType, "", resource.VersionUndefined))
			if err != nil {
				return fmt.Errorf("error listing resources: %w", err)
			}

			for _, res := range list.Items {
				if res.Metadata().Owner() != ctrl.Name() {
					continue
				}

				if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
					if err = r.Destroy(ctx, res.Metadata()); err != nil {
						return fmt.Errorf("error cleaning up specs: %w", err)
					}
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
