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

	clustertypes "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// ConfigController watches v1alpha1.Config, updates discovery config.
type ConfigController = transform.Controller[*config.MachineConfig, *cluster.Config]

// NewConfigController instantiates the config controller.
func NewConfigController() *ConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *cluster.Config]{
			Name: "cluster.ConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*cluster.Config] {
				if cfg.Metadata().ID() != config.ActiveID {
					return optional.None[*cluster.Config]()
				}

				if cfg.Config().Cluster() == nil {
					return optional.None[*cluster.Config]()
				}

				return optional.Some(cluster.NewConfig(config.NamespaceName, cluster.ConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *cluster.Config) error {
				var err error

				cfg := machineConfig.Config()

				// Both the legacy v1alpha1 service endpoint, and the new multi-doc DiscoveryServiceConfig(s) get
				// surfaced via the .DiscoveryServiceConfigs() interface. By now the configs have been validated.
				discoveryServiceConfigs := cfg.DiscoveryServiceConfigs()

				if len(discoveryServiceConfigs) > 0 {
					res.TypedSpec().ServiceEndpoints = []cluster.ServiceEndpoint{}

					for _, discoveryServiceConfig := range discoveryServiceConfigs {
						normalizedEndpoint, insecure, err := NormalizeDiscoveryEndpoint(discoveryServiceConfig.Endpoint().String())
						if err != nil {
							return err
						}

						res.TypedSpec().ServiceEndpoints = append(res.TypedSpec().ServiceEndpoints, cluster.ServiceEndpoint{
							Name:     discoveryServiceConfig.Name(),
							Endpoint: normalizedEndpoint,
							Insecure: insecure,
						})
					}

					res.TypedSpec().ServiceEncryptionKey, err = base64.StdEncoding.DecodeString(cfg.Cluster().Secret())
					if err != nil {
						return err
					}

					res.TypedSpec().ServiceClusterID = cfg.Cluster().ID()

					// Legacy field support for Omni backwards compatibility.
					// We don't actually use these fields anymore in the discovery service controller.
					res.TypedSpec().RegistryServiceEnabled = true //nolint:staticcheck // legacy config
					// Just use the first one off the top. We can't use all of them until Omni supports multiple discovery services.
					// This is the normalized endpoint (just port:host) and insecure flag, not the raw URL.
					res.TypedSpec().ServiceEndpoint = res.TypedSpec().ServiceEndpoints[0].Endpoint         //nolint:staticcheck // legacy config
					res.TypedSpec().ServiceEndpointInsecure = res.TypedSpec().ServiceEndpoints[0].Insecure //nolint:staticcheck // legacy config
				} else {
					res.TypedSpec().ServiceEncryptionKey = nil
					res.TypedSpec().ServiceClusterID = ""
					res.TypedSpec().ServiceEndpoints = nil

					// Legacy field support for Omni backwards compatibility.
					res.TypedSpec().RegistryServiceEnabled = false  //nolint:staticcheck // legacy config
					res.TypedSpec().ServiceEndpoint = ""            //nolint:staticcheck // legacy config
					res.TypedSpec().ServiceEndpointInsecure = false //nolint:staticcheck // legacy config
				}

				// Legacy support for Kubernetes discovery (not discovery service)
				if cfg.Cluster().Discovery().Enabled() {
					res.TypedSpec().RegistryKubernetesEnabled = cfg.Cluster().Discovery().Registries().Kubernetes().Enabled()
				} else {
					res.TypedSpec().RegistryKubernetesEnabled = false
				}

				return nil
			},
		},
	)
}

// NormalizeDiscoveryEndpoint normalizes a discovery service endpoint URL into a host:port address plus insecure flag.
func NormalizeDiscoveryEndpoint(rawEndpoint string) (addr string, insecure bool, err error) {
	u, err := url.Parse(rawEndpoint)
	if err != nil {
		return "", false, err
	}

	if err := clustertypes.ValidateDiscoveryServiceEndpoint(u); err != nil {
		return "", false, err
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}

	return net.JoinHostPort(u.Hostname(), port), u.Scheme == "http", nil
}
