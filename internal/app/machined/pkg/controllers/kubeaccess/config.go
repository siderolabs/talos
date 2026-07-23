// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubeaccess"
)

// ConfigController watches v1alpha1.Config, updates Talos API access config.
type ConfigController = transform.Controller[*config.MachineConfig, *kubeaccess.Config]

// NewConfigController instantiates the config controller.
func NewConfigController() *ConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *kubeaccess.Config]{
			Name: "kubeaccess.ConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*kubeaccess.Config] {
				if cfg.Metadata().ID() != config.ActiveID {
					return optional.None[*kubeaccess.Config]()
				}

				if cfg.Config().Machine() == nil {
					return optional.None[*kubeaccess.Config]()
				}

				if !cfg.Config().Machine().Type().IsControlPlane() {
					return optional.None[*kubeaccess.Config]()
				}

				return optional.Some(kubeaccess.NewConfig(config.NamespaceName, kubeaccess.ConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *kubeaccess.Config) error {
				spec := res.TypedSpec()

				*spec = kubeaccess.ConfigSpec{}

				c := cfg.Config().K8sTalosAPIAccessConfig()

				spec.Enabled = c != nil

				if spec.Enabled {
					spec.AllowedAPIRoles = c.AllowedRoles()
					spec.AllowedKubernetesNamespaces = c.AllowedKubernetesNamespaces()
				} else {
					spec.AllowedAPIRoles = nil
					spec.AllowedKubernetesNamespaces = nil
				}

				return nil
			},
		},
	)
}
