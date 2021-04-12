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
	"net/url"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/secrets"
	timeresource "github.com/talos-systems/talos/pkg/resources/time"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// KubernetesCertificateValidityDuration is the validity duration for the certificates created with this controller.
//
// Controller automatically refreshes certs at 50% of CertificateValidityDuration.
const KubernetesCertificateValidityDuration = constants.KubernetesDefaultCertificateValidityDuration

// KubernetesController manages secrets.Kubernetes based on configuration.
type KubernetesController struct{}

// Name implements controller.Controller interface.
func (ctrl *KubernetesController) Name() string {
	return "secrets.KubernetesController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubernetesController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.RootType,
			ID:        pointer.ToString(secrets.RootKubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("networkd"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeresource.StatusType,
			ID:        pointer.ToString(timeresource.StatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubernetesController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.KubernetesType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KubernetesController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	refreshTicker := time.NewTicker(KubernetesCertificateValidityDuration / 2)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-refreshTicker.C:
		}

		k8sRootRes, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.RootType, secrets.RootKubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting root k8s secrets: %w", err)
		}

		k8sRoot := k8sRootRes.(*secrets.Root).KubernetesSpec()

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
		timeSyncResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, timeresource.StatusType, timeresource.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !timeSyncResource.(*timeresource.Status).Status().Synced {
			continue
		}

		if err = r.Modify(ctx, secrets.NewKubernetes(), func(r resource.Resource) error {
			return ctrl.updateSecrets(k8sRoot, r.(*secrets.Kubernetes).Certs())
		}); err != nil {
			return err
		}
	}
}

func (ctrl *KubernetesController) updateSecrets(k8sRoot *secrets.RootKubernetesSpec, k8sSecrets *secrets.KubernetesCertsSpec) error {
	urls := []string{k8sRoot.Endpoint.Hostname()}
	urls = append(urls, k8sRoot.CertSANs...)
	altNames := altNamesFromURLs(urls)

	altNames.IPs = append(altNames.IPs, k8sRoot.APIServerIPs...)

	// Add kubernetes default svc with cluster domain to AltNames
	altNames.DNSNames = append(altNames.DNSNames,
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc."+k8sRoot.DNSDomain,
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(k8sRoot.CA)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	apiServer, err := x509.NewKeyPair(ca,
		x509.IPAddresses(altNames.IPs),
		x509.DNSNames(altNames.DNSNames),
		x509.CommonName("kube-apiserver"),
		x509.Organization("kube-master"),
		x509.NotAfter(time.Now().Add(KubernetesCertificateValidityDuration)),
	)
	if err != nil {
		return fmt.Errorf("failed to generate api-server cert: %w", err)
	}

	k8sSecrets.APIServer = x509.NewCertificateAndKeyFromKeyPair(apiServer)

	apiServerKubeletClient, err := x509.NewKeyPair(ca,
		x509.CommonName(constants.KubernetesAdminCertCommonName),
		x509.Organization(constants.KubernetesAdminCertOrganization),
		x509.NotAfter(time.Now().Add(KubernetesCertificateValidityDuration)),
	)
	if err != nil {
		return fmt.Errorf("failed to generate api-server cert: %w", err)
	}

	k8sSecrets.APIServerKubeletClient = x509.NewCertificateAndKeyFromKeyPair(apiServerKubeletClient)

	aggregatorCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(k8sRoot.AggregatorCA)
	if err != nil {
		return fmt.Errorf("failed to parse aggregator CA: %w", err)
	}

	frontProxy, err := x509.NewKeyPair(aggregatorCA,
		x509.CommonName("front-proxy-client"),
		x509.NotAfter(time.Now().Add(KubernetesCertificateValidityDuration)),
	)
	if err != nil {
		return fmt.Errorf("failed to generate aggregator cert: %w", err)
	}

	k8sSecrets.FrontProxy = x509.NewCertificateAndKeyFromKeyPair(frontProxy)

	var buf bytes.Buffer

	if err = kubeconfig.GenerateAdmin(&generateAdminAdapter{k8sRoot: k8sRoot}, &buf); err != nil {
		return fmt.Errorf("failed to generate admin kubeconfig: %w", err)
	}

	k8sSecrets.AdminKubeconfig = buf.String()

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

// generateAdminAdapter allows to translate input config into GenerateAdmin input.
type generateAdminAdapter struct {
	k8sRoot *secrets.RootKubernetesSpec
}

func (adapter *generateAdminAdapter) Name() string {
	return adapter.k8sRoot.Name
}

func (adapter *generateAdminAdapter) Endpoint() *url.URL {
	return adapter.k8sRoot.Endpoint
}

func (adapter *generateAdminAdapter) CA() *x509.PEMEncodedCertificateAndKey {
	return adapter.k8sRoot.CA
}

func (adapter *generateAdminAdapter) AdminKubeconfig() config.AdminKubeconfig {
	return adapter
}

func (adapter *generateAdminAdapter) CertLifetime() time.Duration {
	// this certificate is not delivered to the user, it's used only internally by control plane components
	return KubernetesCertificateValidityDuration
}
