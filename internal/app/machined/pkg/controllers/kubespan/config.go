// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

// ConfigController watches v1alpha1.Config, updates KubeSpan config.
type ConfigController = transform.Controller[*config.MachineConfig, *kubespan.Config]

// NewConfigController instanciates the config controller.
func NewConfigController() *ConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *kubespan.Config]{
			Name: "kubespan.ConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*kubespan.Config] {
				if cfg.Metadata().ID() != config.ActiveID {
					return optional.None[*kubespan.Config]()
				}

				if cfg.Config().Machine() == nil || cfg.Config().Cluster() == nil {
					return optional.None[*kubespan.Config]()
				}

				return optional.Some(kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *kubespan.Config) error {
				spec := res.TypedSpec()

				*spec = kubespan.ConfigSpec{}

				if cfg != nil && cfg.Config().Machine() != nil {
					c := cfg.Config()

					if c.NetworkKubeSpanConfig() != nil {
						res.TypedSpec().Enabled = c.NetworkKubeSpanConfig().Enabled()
						res.TypedSpec().ForceRouting = c.NetworkKubeSpanConfig().ForceRouting()
						res.TypedSpec().AdvertiseKubernetesNetworks = c.NetworkKubeSpanConfig().AdvertiseKubernetesNetworks()
						res.TypedSpec().HarvestExtraEndpoints = c.NetworkKubeSpanConfig().HarvestExtraEndpoints()
						res.TypedSpec().MTU = c.NetworkKubeSpanConfig().MTU()

						if c.NetworkKubeSpanConfig().Filters() != nil {
							res.TypedSpec().EndpointFilters = c.NetworkKubeSpanConfig().Filters().Endpoints()
						}
					}

					res.TypedSpec().ClusterID = c.Cluster().ID()
					res.TypedSpec().SharedSecret = c.Cluster().Secret()
					res.TypedSpec().ExtraEndpoints = c.KubespanConfig().ExtraAnnouncedEndpoints()
				}

				return nil
			},
		},
	)
}
