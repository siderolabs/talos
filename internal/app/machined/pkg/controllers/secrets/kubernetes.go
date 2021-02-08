// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"

	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/config"
	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/secrets"
	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/v1alpha1"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// KubernetesController manages secrets.Kubernetes based on configuration.
type KubernetesController struct {
}

// Name implements controller.Controller interface.
func (ctrl *KubernetesController) Name() string {
	return "secrets.KubernetesController"
}

// ManagedResources implements controller.Controller interface.
func (ctrl *KubernetesController) ManagedResources() (resource.Namespace, resource.Type) {
	return secrets.NamespaceName, secrets.KubernetesType
}

// Run implements controller.Controller interface.
//
//nolint: gocyclo
func (ctrl *KubernetesController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if err := r.UpdateDependencies([]controller.Dependency{
		{ // TODO: should render config for kubernetes secrets controller
			Namespace: config.NamespaceName,
			Type:      config.V1Alpha1Type,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.DependencyWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("networkd"),
			Kind:      controller.DependencyWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.TimeSyncType,
			ID:        pointer.ToString(v1alpha1.TimeSyncID),
			Kind:      controller.DependencyWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.ToString(config.MachineTypeID),
			Kind:      controller.DependencyWeak,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.V1Alpha1Type, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying static pods: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.(*config.V1Alpha1).Config()

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

		if machineType != machine.TypeControlPlane && machineType != machine.TypeInit {
			if err = ctrl.teardownAll(ctx, r); err != nil {
				return fmt.Errorf("error destroying static pods: %w", err)
			}

			continue
		}

		// wait for networkd to be healthy as it might change IPs/hostname
		networkdResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "networkd", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !networkdResource.(*v1alpha1.Service).Healthy() {
			continue
		}

		// wait for time sync as certs depend on current time
		timeSyncResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.TimeSyncType, v1alpha1.TimeSyncID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !timeSyncResource.(*v1alpha1.TimeSync).Sync() {
			continue
		}

		if err = r.Update(ctx, secrets.NewKubernetes(), func(r resource.Resource) error {
			k8sSecrets := r.(*secrets.Kubernetes) //nolint: errcheck

			return ctrl.updateSecrets(cfgProvider, k8sSecrets)
		}); err != nil {
			return err
		}
	}
}

//nolint: gocyclo
func (ctrl *KubernetesController) updateSecrets(cfgProvider talosconfig.Provider, k8sSecrets *secrets.Kubernetes) error {
	k8sSecrets.Secrets().EtcdCA = cfgProvider.Cluster().Etcd().CA()

	if k8sSecrets.Secrets().EtcdCA == nil {
		return fmt.Errorf("missing cluster.etcdCA secret")
	}

	k8sSecrets.Secrets().AggregatorCA = cfgProvider.Cluster().AggregatorCA()

	if k8sSecrets.Secrets().AggregatorCA == nil {
		return fmt.Errorf("missing cluster.aggregatorCA secret")
	}

	k8sSecrets.Secrets().CA = cfgProvider.Cluster().CA()

	if k8sSecrets.Secrets().CA == nil {
		return fmt.Errorf("missing cluster.CA secret")
	}

	var err error

	k8sSecrets.Secrets().EtcdPeer, err = etcd.GeneratePeerCert(cfgProvider.Cluster().Etcd().CA())
	if err != nil {
		return err
	}

	urls := []string{cfgProvider.Cluster().Endpoint().Hostname()}
	urls = append(urls, cfgProvider.Cluster().CertSANs()...)
	altNames := altNamesFromURLs(urls)

	apiServiceIPs, err := cfgProvider.Cluster().Network().APIServerIPs()
	if err != nil {
		return fmt.Errorf("failed to calculate API service IP: %w", err)
	}

	altNames.IPs = append(altNames.IPs, apiServiceIPs...)

	// Add kubernetes default svc with cluster domain to AltNames
	altNames.DNSNames = append(altNames.DNSNames,
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc."+cfgProvider.Cluster().Network().DNSDomain(),
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(k8sSecrets.Secrets().CA)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	apiServer, err := x509.NewKeyPair(ca,
		x509.IPAddresses(altNames.IPs),
		x509.DNSNames(altNames.DNSNames),
		x509.CommonName("kube-apiserver"),
		x509.Organization("kube-master"),
		x509.NotAfter(time.Now().Add(constants.KubernetesDefaultCertificateValidityDuration)),
	)
	if err != nil {
		return fmt.Errorf("failed to generate api-server cert: %w", err)
	}

	k8sSecrets.Secrets().APIServer = x509.NewCertificateAndKeyFromKeyPair(apiServer)

	apiServerKubeletClient, err := x509.NewKeyPair(ca,
		x509.CommonName(constants.KubernetesAdminCertCommonName),
		x509.Organization(constants.KubernetesAdminCertOrganization),
		x509.NotAfter(time.Now().Add(constants.KubernetesDefaultCertificateValidityDuration)),
	)
	if err != nil {
		return fmt.Errorf("failed to generate api-server cert: %w", err)
	}

	k8sSecrets.Secrets().APIServerKubeletClient = x509.NewCertificateAndKeyFromKeyPair(apiServerKubeletClient)

	k8sSecrets.Secrets().ServiceAccount = cfgProvider.Cluster().ServiceAccount()

	aggregatorCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(k8sSecrets.Secrets().AggregatorCA)
	if err != nil {
		return fmt.Errorf("failed to parse aggregator CA: %w", err)
	}

	frontProxy, err := x509.NewKeyPair(aggregatorCA,
		x509.CommonName("front-proxy-client"),
		x509.NotAfter(time.Now().Add(constants.KubernetesDefaultCertificateValidityDuration)),
	)
	if err != nil {
		return fmt.Errorf("failed to generate aggregator cert: %w", err)
	}

	k8sSecrets.Secrets().FrontProxy = x509.NewCertificateAndKeyFromKeyPair(frontProxy)

	k8sSecrets.Secrets().AESCBCEncryptionSecret = cfgProvider.Cluster().AESCBCEncryptionSecret()

	var buf bytes.Buffer

	if err = kubeconfig.GenerateAdmin(cfgProvider.Cluster(), &buf); err != nil {
		return fmt.Errorf("failed to generate admin kubeconfig: %w", err)
	}

	k8sSecrets.Secrets().AdminKubeconfig = buf.String()

	k8sSecrets.Secrets().BootstrapTokenID = cfgProvider.Cluster().Token().ID()
	k8sSecrets.Secrets().BootstrapTokenSecret = cfgProvider.Cluster().Token().Secret()

	return nil
}

func (ctrl *KubernetesController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	// TODO: change this to proper teardown sequence

	for _, res := range list.Items {
		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}

// AltNames defines certificate alternative names.
type AltNames struct {
	IPs      []net.IP
	DNSNames []string
}

func altNamesFromURLs(urls []string) *AltNames {
	var an AltNames

	for _, u := range urls {
		ip := net.ParseIP(u)
		if ip != nil {
			an.IPs = append(an.IPs, ip)

			continue
		}

		an.DNSNames = append(an.DNSNames, u)
	}

	return &an
}
