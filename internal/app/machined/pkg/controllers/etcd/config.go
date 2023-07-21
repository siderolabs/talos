// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// ConfigController watches v1alpha1.Config, updates etcd config.
type ConfigController = transform.Controller[*config.MachineConfig, *etcd.Config]

// NewConfigController instanciates the config controller.
func NewConfigController() *ConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *etcd.Config]{
			Name: "etcd.ConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*etcd.Config] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*etcd.Config]()
				}

				if cfg.Config().Machine() == nil || cfg.Config().Cluster() == nil {
					return optional.None[*etcd.Config]()
				}

				if !cfg.Config().Machine().Type().IsControlPlane() {
					// etcd only runs on controlplane nodes
					return optional.None[*etcd.Config]()
				}

				return optional.Some(etcd.NewConfig(etcd.NamespaceName, etcd.ConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, cfg *etcd.Config) error {
				cfg.TypedSpec().AdvertiseValidSubnets = machineConfig.Config().Cluster().Etcd().AdvertisedSubnets()
				cfg.TypedSpec().AdvertiseExcludeSubnets = nil
				cfg.TypedSpec().ListenValidSubnets = machineConfig.Config().Cluster().Etcd().ListenSubnets()
				cfg.TypedSpec().ListenExcludeSubnets = nil

				// filter out any virtual IPs, they can't be node IPs either
				for _, device := range machineConfig.Config().Machine().Network().Devices() {
					if device.VIPConfig() != nil {
						cfg.TypedSpec().AdvertiseExcludeSubnets = append(cfg.TypedSpec().AdvertiseExcludeSubnets, device.VIPConfig().IP())
					}

					for _, vlan := range device.Vlans() {
						if vlan.VIPConfig() != nil {
							cfg.TypedSpec().AdvertiseExcludeSubnets = append(cfg.TypedSpec().AdvertiseExcludeSubnets, vlan.VIPConfig().IP())
						}
					}
				}

				cfg.TypedSpec().Image = machineConfig.Config().Cluster().Etcd().Image()
				cfg.TypedSpec().ExtraArgs = machineConfig.Config().Cluster().Etcd().ExtraArgs()

				return nil
			},
		},
	)
}
