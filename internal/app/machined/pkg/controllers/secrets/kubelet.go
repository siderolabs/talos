// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// KubeletController manages secrets.Kubelet based on configuration.
type KubeletController = transform.Controller[*config.MachineConfig, *secrets.Kubelet]

// NewKubeletController instanciates the controller.
func NewKubeletController() *KubeletController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *secrets.Kubelet]{
			Name: "secrets.KubeletController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*secrets.Kubelet] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*secrets.Kubelet]()
				}

				if cfg.Config().Cluster() == nil || cfg.Config().Machine() == nil {
					return optional.None[*secrets.Kubelet]()
				}

				return optional.Some(secrets.NewKubelet(secrets.KubeletID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *secrets.Kubelet) error {
				cfgProvider := cfg.Config()
				kubeletSecrets := res.TypedSpec()

				switch {
				case cfgProvider.Machine().Features().KubePrism().Enabled():
					// use cluster endpoint for controlplane nodes with loadbalancer support
					localEndpoint, err := url.Parse(fmt.Sprintf("https://127.0.0.1:%d", cfgProvider.Machine().Features().KubePrism().Port()))
					if err != nil {
						return err
					}

					kubeletSecrets.Endpoint = localEndpoint
				case cfgProvider.Machine().Type().IsControlPlane():
					// use localhost endpoint for controlplane nodes
					localEndpoint, err := url.Parse(fmt.Sprintf("https://localhost:%d", cfgProvider.Cluster().LocalAPIServerPort()))
					if err != nil {
						return err
					}

					kubeletSecrets.Endpoint = localEndpoint
				default:
					// use cluster endpoint for workers
					kubeletSecrets.Endpoint = cfgProvider.Cluster().Endpoint()
				}

				kubeletSecrets.AcceptedCAs = nil

				if cfgProvider.Cluster().IssuingCA() != nil {
					kubeletSecrets.AcceptedCAs = append(kubeletSecrets.AcceptedCAs, &x509.PEMEncodedCertificate{Crt: cfgProvider.Cluster().IssuingCA().Crt})
				}

				kubeletSecrets.AcceptedCAs = append(kubeletSecrets.AcceptedCAs, cfgProvider.Cluster().AcceptedCAs()...)

				if len(kubeletSecrets.AcceptedCAs) == 0 {
					return errors.New("missing accepted Kubernetes CAs")
				}

				kubeletSecrets.BootstrapTokenID = cfgProvider.Cluster().Token().ID()
				kubeletSecrets.BootstrapTokenSecret = cfgProvider.Cluster().Token().Secret()

				return nil
			},
		},
	)
}
