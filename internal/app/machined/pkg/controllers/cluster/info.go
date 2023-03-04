// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// InfoController looks up control plane infos.
type InfoController struct{}

// Name implements controller.Controller interface.
func (ctrl *InfoController) Name() string {
	return "cluster.InfoController"
}

// Inputs implements controller.Controller interface.
func (ctrl *InfoController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *InfoController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.InfoType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *InfoController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		err = safe.WriterModify(ctx, r, cluster.NewInfo(), func(info *cluster.Info) error {
			info.TypedSpec().ClusterID = cfg.Config().Cluster().ID()
			info.TypedSpec().ClusterName = cfg.Config().Cluster().Name()

			return nil
		})
		if err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
