// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// InfoController looks up control plane infos.
type InfoController = transform.Controller[*config.MachineConfig, *cluster.Info]

// NewInfoController instanciates the cluster info controller.
func NewInfoController() *InfoController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *cluster.Info]{
			Name: "cluster.InfoController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*cluster.Info] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*cluster.Info]()
				}

				if cfg.Config().Cluster() == nil {
					return optional.None[*cluster.Info]()
				}

				return optional.Some(cluster.NewInfo())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, info *cluster.Info) error {
				info.TypedSpec().ClusterID = cfg.Config().Cluster().ID()
				info.TypedSpec().ClusterName = cfg.Config().Cluster().Name()

				return nil
			},
		},
	)
}
