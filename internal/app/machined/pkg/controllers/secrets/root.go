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
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func rootMapFunc[Output generic.ResourceWithRD](
	output Output,
	requireControlPlane bool,
	extraChecks ...func(cfg *config.MachineConfig) bool,
) func(cfg *config.MachineConfig) optional.Optional[Output] {
	return func(cfg *config.MachineConfig) optional.Optional[Output] {
		if cfg.Metadata().ID() != config.ActiveID {
			return optional.None[Output]()
		}

		if cfg.Config().Cluster() == nil || cfg.Config().Machine() == nil {
			return optional.None[Output]()
		}

		if requireControlPlane && !cfg.Config().Machine().Type().IsControlPlane() {
			return optional.None[Output]()
		}

		for _, check := range extraChecks {
			if !check(cfg) {
				return optional.None[Output]()
			}
		}

		return optional.Some(output)
	}
}

// RootEtcdController manages secrets.EtcdRoot based on configuration.
type RootEtcdController = transform.Controller[*config.MachineConfig, *secrets.EtcdRoot]

// NewRootEtcdController instantiates the controller.
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

// NewRootKubernetesController instantiates the controller.
func NewRootKubernetesController() *RootKubernetesController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *secrets.KubernetesRoot]{
			Name: "secrets.RootKubernetesController",
			MapMetadataOptionalFunc: rootMapFunc(
				secrets.NewKubernetesRoot(secrets.KubernetesRootID),
				true,
				func(cfg *config.MachineConfig) bool {
					return cfg.Config().K8sAPIServerConfig() != nil
				},
				func(cfg *config.MachineConfig) bool {
					return cfg.Config().K8sAPIServerCAConfig() != nil
				},
				func(cfg *config.MachineConfig) bool {
					return cfg.Config().K8sAggregatorCAConfig() != nil
				},
				func(cfg *config.MachineConfig) bool {
					return cfg.Config().K8sServiceAccountConfig() != nil
				},
				func(cfg *config.MachineConfig) bool {
					return cfg.Config().K8sClusterConfig() != nil
				},
			),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *secrets.KubernetesRoot) error {
				cfgProvider := cfg.Config()
				k8sSecrets := res.TypedSpec()

				var (
					err           error
					localEndpoint *url.URL
				)

				if kubePrismConfig := cfgProvider.K8sKubePrismConfig(); kubePrismConfig != nil {
					localEndpoint, err = url.Parse(fmt.Sprintf("https://127.0.0.1:%d", kubePrismConfig.Port()))
					if err != nil {
						return err
					}
				} else {
					localEndpoint, err = url.Parse(fmt.Sprintf("https://localhost:%d", cfgProvider.K8sAPIServerConfig().APIPort()))
					if err != nil {
						return err
					}
				}

				k8sSecrets.Name = cfgProvider.K8sClusterConfig().ClusterName()
				k8sSecrets.Endpoint = cfgProvider.K8sClusterConfig().ClusterEndpoint()
				k8sSecrets.LocalEndpoint = localEndpoint
				k8sSecrets.CertSANs = cfgProvider.K8sAPIServerConfig().CertSANs()

				if k8sNetwork := cfgProvider.K8sNetworkConfig(); k8sNetwork != nil {
					k8sSecrets.DNSDomain = k8sNetwork.DNSDomain()
					k8sSecrets.APIServerIPs = k8s.APIServerServiceAddrs(k8sNetwork.ServiceCIDRs())
				} else {
					k8sSecrets.DNSDomain = ""
					k8sSecrets.APIServerIPs = nil
				}

				k8sSecrets.AggregatorCA = cfgProvider.K8sAggregatorCAConfig().IssuingCA()
				k8sSecrets.AcceptedAggregatorCAs = cfgProvider.K8sAggregatorCAConfig().AcceptedCAs()

				k8sSecrets.IssuingCA = cfgProvider.K8sAPIServerCAConfig().IssuingCA()
				k8sSecrets.AcceptedCAs = cfgProvider.K8sAPIServerCAConfig().AcceptedCAs()

				if len(k8sSecrets.AcceptedCAs) == 0 {
					return errors.New("missing cluster.CA secret")
				}

				k8sSecrets.ServiceAccount = cfgProvider.K8sServiceAccountConfig().IssuingKey()
				k8sSecrets.ServiceAccountAcceptedKeys = cfgProvider.K8sServiceAccountConfig().AcceptedKeys()
				k8sSecrets.IssuerURL = cfgProvider.K8sServiceAccountConfig().IssuerURL()
				k8sSecrets.AcceptedIssuers = cfgProvider.K8sServiceAccountConfig().AcceptedIssuers()
				k8sSecrets.APIAudiences = cfgProvider.K8sServiceAccountConfig().APIAudiences()

				k8sSecrets.AESCBCEncryptionSecret = cfgProvider.Cluster().AESCBCEncryptionSecret()
				k8sSecrets.SecretboxEncryptionSecret = cfgProvider.Cluster().SecretboxEncryptionSecret()

				k8sSecrets.BootstrapTokenID = cfgProvider.Cluster().Token().ID()
				k8sSecrets.BootstrapTokenSecret = cfgProvider.Cluster().Token().Secret()

				if etcdEncryptionConfig := cfgProvider.K8sEtcdEncryptionConfig(); etcdEncryptionConfig != nil {
					k8sSecrets.EtcdEncryptionConfig = etcdEncryptionConfig.EtcdEncryptionConfig()
				} else {
					k8sSecrets.EtcdEncryptionConfig = nil
				}

				return nil
			},
		},
	)
}

// RootOSController manages secrets.OSRoot based on configuration.
type RootOSController = transform.Controller[*config.MachineConfig, *secrets.OSRoot]

// NewRootOSController instantiates the controller.
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

				if cfgProvider.K8sTalosAPIAccessConfig() != nil {
					// add Kubernetes Talos service name to the list of SANs
					osSecrets.CertSANDNSNames = append(
						osSecrets.CertSANDNSNames,
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
