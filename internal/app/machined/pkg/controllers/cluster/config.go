// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"encoding/base64"
	"net"
	"net/url"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// ConfigController watches v1alpha1.Config, updates discovery config.
type ConfigController = transform.Controller[*config.MachineConfig, *cluster.Config]

// NewConfigController instanciates the config controller.
func NewConfigController() *ConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *cluster.Config]{
			Name: "cluster.ConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*cluster.Config] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*cluster.Config]()
				}

				if cfg.Config().Cluster() == nil {
					return optional.None[*cluster.Config]()
				}

				return optional.Some(cluster.NewConfig(config.NamespaceName, cluster.ConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *cluster.Config) error {
				c := cfg.Config()

				res.TypedSpec().DiscoveryEnabled = c.Cluster().Discovery().Enabled()

				if c.Cluster().Discovery().Enabled() {
					res.TypedSpec().RegistryKubernetesEnabled = c.Cluster().Discovery().Registries().Kubernetes().Enabled()
					res.TypedSpec().RegistryServiceEnabled = c.Cluster().Discovery().Registries().Service().Enabled()

					if c.Cluster().Discovery().Registries().Service().Enabled() {
						var u *url.URL

						u, err := url.ParseRequestURI(c.Cluster().Discovery().Registries().Service().Endpoint())
						if err != nil {
							return err
						}

						host := u.Hostname()
						port := u.Port()

						if port == "" {
							if u.Scheme == "http" {
								port = "80"
							} else {
								port = "443" // use default https port for everything else
							}
						}

						res.TypedSpec().ServiceEndpoint = net.JoinHostPort(host, port)
						res.TypedSpec().ServiceEndpointInsecure = u.Scheme == "http"

						res.TypedSpec().ServiceEncryptionKey, err = base64.StdEncoding.DecodeString(c.Cluster().Secret())
						if err != nil {
							return err
						}

						res.TypedSpec().ServiceClusterID = c.Cluster().ID()
					} else {
						res.TypedSpec().ServiceEndpoint = ""
						res.TypedSpec().ServiceEndpointInsecure = false
						res.TypedSpec().ServiceEncryptionKey = nil
						res.TypedSpec().ServiceClusterID = ""
					}
				} else {
					res.TypedSpec().RegistryKubernetesEnabled = false
					res.TypedSpec().RegistryServiceEnabled = false
				}

				return nil
			},
		},
	)
}
