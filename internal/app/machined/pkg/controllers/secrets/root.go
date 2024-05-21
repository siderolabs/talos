// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"net/url"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func rootMapFunc[Output generic.ResourceWithRD](output Output, requireControlPlane bool) func(cfg *config.MachineConfig) optional.Optional[Output] {
	return func(cfg *config.MachineConfig) optional.Optional[Output] {
		if cfg.Metadata().ID() != config.V1Alpha1ID {
			return optional.None[Output]()
		}

		if cfg.Config().Cluster() == nil || cfg.Config().Machine() == nil {
			return optional.None[Output]()
		}

		if requireControlPlane && !cfg.Config().Machine().Type().IsControlPlane() {
			return optional.None[Output]()
		}

		return optional.Some(output)
	}
}

// RootEtcdController manages secrets.EtcdRoot based on configuration.
type RootEtcdController = transform.Controller[*config.MachineConfig, *secrets.EtcdRoot]

// NewRootEtcdController instanciates the controller.
func NewRootEtcdController() *RootEtcdController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *secrets.EtcdRoot]{
			Name:                    "secrets.RootEtcdController",
			MapMetadataOptionalFunc: rootMapFunc(secrets.NewEtcdRoot(secrets.EtcdRootID), true),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *secrets.EtcdRoot) error {
				cfgProvider := cfg.Config()
				etcdSecrets := res.TypedSpec()

				etcdSecrets.EtcdCA = cfgProvider.Cluster().Etcd().CA()

				if etcdSecrets.EtcdCA == nil {
					return errors.New("missing cluster.etcdCA secret")
				}

				return nil
			},
		},
	)
}

// RootKubernetesController manages secrets.KubernetesRoot based on configuration.
type RootKubernetesController = transform.Controller[*config.MachineConfig, *secrets.KubernetesRoot]

// NewRootKubernetesController instanciates the controller.
func NewRootKubernetesController() *RootKubernetesController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *secrets.KubernetesRoot]{
			Name:                    "secrets.RootKubernetesController",
			MapMetadataOptionalFunc: rootMapFunc(secrets.NewKubernetesRoot(secrets.KubernetesRootID), true),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *secrets.KubernetesRoot) error {
				cfgProvider := cfg.Config()
				k8sSecrets := res.TypedSpec()

				var (
					err           error
					localEndpoint *url.URL
				)

				if cfgProvider.Machine().Features().KubePrism().Enabled() {
					localEndpoint, err = url.Parse(fmt.Sprintf("https://127.0.0.1:%d", cfgProvider.Machine().Features().KubePrism().Port()))
					if err != nil {
						return err
					}
				} else {
					localEndpoint, err = url.Parse(fmt.Sprintf("https://localhost:%d", cfgProvider.Cluster().LocalAPIServerPort()))
					if err != nil {
						return err
					}
				}

				k8sSecrets.Name = cfgProvider.Cluster().Name()
				k8sSecrets.Endpoint = cfgProvider.Cluster().Endpoint()
				k8sSecrets.LocalEndpoint = localEndpoint
				k8sSecrets.CertSANs = cfgProvider.Cluster().CertSANs()
				k8sSecrets.DNSDomain = cfgProvider.Cluster().Network().DNSDomain()

				k8sSecrets.APIServerIPs, err = cfgProvider.Cluster().Network().APIServerIPs()
				if err != nil {
					return fmt.Errorf("error building API service IPs: %w", err)
				}

				k8sSecrets.AggregatorCA = cfgProvider.Cluster().AggregatorCA()

				if k8sSecrets.AggregatorCA == nil {
					return errors.New("missing cluster.aggregatorCA secret")
				}

				k8sSecrets.IssuingCA = cfgProvider.Cluster().IssuingCA()
				k8sSecrets.AcceptedCAs = cfgProvider.Cluster().AcceptedCAs()

				if k8sSecrets.IssuingCA != nil {
					k8sSecrets.AcceptedCAs = append(k8sSecrets.AcceptedCAs, &x509.PEMEncodedCertificate{
						Crt: k8sSecrets.IssuingCA.Crt,
					})
				}

				if len(k8sSecrets.IssuingCA.Key) == 0 {
					// drop incomplete issuing CA, as the machine config for workers contains just the cert
					k8sSecrets.IssuingCA = nil
				}

				if len(k8sSecrets.AcceptedCAs) == 0 {
					return errors.New("missing cluster.CA secret")
				}

				k8sSecrets.ServiceAccount = cfgProvider.Cluster().ServiceAccount()

				k8sSecrets.AESCBCEncryptionSecret = cfgProvider.Cluster().AESCBCEncryptionSecret()
				k8sSecrets.SecretboxEncryptionSecret = cfgProvider.Cluster().SecretboxEncryptionSecret()

				k8sSecrets.BootstrapTokenID = cfgProvider.Cluster().Token().ID()
				k8sSecrets.BootstrapTokenSecret = cfgProvider.Cluster().Token().Secret()

				return nil
			},
		},
	)
}

// RootOSController manages secrets.OSRoot based on configuration.
type RootOSController = transform.Controller[*config.MachineConfig, *secrets.OSRoot]

// NewRootOSController instanciates the controller.
func NewRootOSController() *RootOSController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *secrets.OSRoot]{
			Name:                    "secrets.RootOSController",
			MapMetadataOptionalFunc: rootMapFunc(secrets.NewOSRoot(secrets.OSRootID), false),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *secrets.OSRoot) error {
				cfgProvider := cfg.Config()
				osSecrets := res.TypedSpec()

				osSecrets.IssuingCA = cfgProvider.Machine().Security().IssuingCA()
				osSecrets.AcceptedCAs = cfgProvider.Machine().Security().AcceptedCAs()

				if osSecrets.IssuingCA != nil {
					osSecrets.AcceptedCAs = append(osSecrets.AcceptedCAs, &x509.PEMEncodedCertificate{
						Crt: osSecrets.IssuingCA.Crt,
					})

					if len(osSecrets.IssuingCA.Key) == 0 {
						// drop incomplete issuing CA, as the machine config for workers contains just the cert
						osSecrets.IssuingCA = nil
					}
				}

				osSecrets.CertSANIPs = nil
				osSecrets.CertSANDNSNames = nil

				for _, san := range cfgProvider.Machine().Security().CertSANs() {
					if ip, err := netip.ParseAddr(san); err == nil {
						osSecrets.CertSANIPs = append(osSecrets.CertSANIPs, ip)
					} else {
						osSecrets.CertSANDNSNames = append(osSecrets.CertSANDNSNames, san)
					}
				}

				if cfgProvider.Machine().Features().KubernetesTalosAPIAccess().Enabled() {
					// add Kubernetes Talos service name to the list of SANs
					osSecrets.CertSANDNSNames = append(osSecrets.CertSANDNSNames,
						constants.KubernetesTalosAPIServiceName,
						constants.KubernetesTalosAPIServiceName+"."+constants.KubernetesTalosAPIServiceNamespace,
					)
				}

				osSecrets.Token = cfgProvider.Machine().Security().Token()

				return nil
			},
		},
	)
}
