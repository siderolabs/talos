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
				if cfg.Metadata().ID() != config.V1Alpha1ID {
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

					res.TypedSpec().Enabled = c.Machine().Network().KubeSpan().Enabled()
					res.TypedSpec().ClusterID = c.Cluster().ID()
					res.TypedSpec().SharedSecret = c.Cluster().Secret()
					res.TypedSpec().ForceRouting = c.Machine().Network().KubeSpan().ForceRouting()
					res.TypedSpec().AdvertiseKubernetesNetworks = c.Machine().Network().KubeSpan().AdvertiseKubernetesNetworks()
					res.TypedSpec().HarvestExtraEndpoints = c.Machine().Network().KubeSpan().HarvestExtraEndpoints()
					res.TypedSpec().MTU = c.Machine().Network().KubeSpan().MTU()
					res.TypedSpec().EndpointFilters = c.Machine().Network().KubeSpan().Filters().Endpoints()
					res.TypedSpec().ExtraEndpoints = c.KubespanConfig().ExtraAnnouncedEndpoints()
				}

				return nil
			},
		},
	)
}
