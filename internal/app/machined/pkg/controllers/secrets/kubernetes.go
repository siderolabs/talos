// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"bytes"
	"context"
	stdlibx509 "crypto/x509"
	"fmt"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/kubeconfig"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/talos-systems/talos/pkg/machinery/resources/time"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
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
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        pointer.To(network.StatusID),
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
//nolint:gocyclo,cyclop
func (ctrl *KubernetesController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// wait for the network to be ready first, then switch to regular inputs
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}
		// wait for network to be ready as it might change IPs/hostname
		networkResource, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.StatusType, network.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		networkStatus := networkResource.(*network.Status).TypedSpec()

		if networkStatus.AddressReady && networkStatus.HostnameReady {
			break
		}
	}

	// switch to regular inputs once the network is ready
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        pointer.To(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeresource.StatusType,
			ID:        pointer.To(timeresource.StatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.CertSANType,
			ID:        pointer.To(secrets.CertSANKubernetesID),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return fmt.Errorf("error updating inputs: %w", err)
	}

	r.QueueReconcile()

	refreshTicker := time.NewTicker(KubernetesCertificateValidityDuration / 2)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-refreshTicker.C:
		}

		k8sRootRes, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting root k8s secrets: %w", err)
		}

		k8sRoot := k8sRootRes.(*secrets.KubernetesRoot).TypedSpec()

		// wait for time sync as certs depend on current time
		timeSyncResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, timeresource.StatusType, timeresource.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !timeSyncResource.(*timeresource.Status).TypedSpec().Synced {
			continue
		}

		certSANResource, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.CertSANType, secrets.CertSANKubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		certSANs := certSANResource.(*secrets.CertSAN).TypedSpec()

		if err = r.Modify(ctx, secrets.NewKubernetes(), func(r resource.Resource) error {
			return ctrl.updateSecrets(k8sRoot, r.(*secrets.Kubernetes).TypedSpec(), certSANs)
		}); err != nil {
			return err
		}
	}
}

func (ctrl *KubernetesController) updateSecrets(k8sRoot *secrets.KubernetesRootSpec, k8sSecrets *secrets.KubernetesCertsSpec,
	certSANs *secrets.CertSANSpec,
) error {
	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(k8sRoot.CA)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	apiServer, err := x509.NewKeyPair(ca,
		x509.IPAddresses(certSANs.StdIPs()),
		x509.DNSNames(certSANs.DNSNames),
		x509.CommonName("kube-apiserver"),
		x509.Organization("kube-master"),
		x509.NotAfter(time.Now().Add(KubernetesCertificateValidityDuration)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageServerAuth,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to generate api-server cert: %w", err)
	}

	k8sSecrets.APIServer = x509.NewCertificateAndKeyFromKeyPair(apiServer)

	apiServerKubeletClient, err := x509.NewKeyPair(ca,
		x509.CommonName(constants.KubernetesAPIServerKubeletClientCommonName),
		x509.Organization(constants.KubernetesAdminCertOrganization),
		x509.NotAfter(time.Now().Add(KubernetesCertificateValidityDuration)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
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
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to generate aggregator cert: %w", err)
	}

	k8sSecrets.FrontProxy = x509.NewCertificateAndKeyFromKeyPair(frontProxy)

	var buf bytes.Buffer

	if err = kubeconfig.Generate(&kubeconfig.GenerateInput{
		ClusterName: k8sRoot.Name,

		CA:                  k8sRoot.CA,
		CertificateLifetime: KubernetesCertificateValidityDuration,

		CommonName:   constants.KubernetesControllerManagerOrganization,
		Organization: constants.KubernetesControllerManagerOrganization,

		Endpoint:    k8sRoot.LocalEndpoint.String(),
		Username:    constants.KubernetesControllerManagerOrganization,
		ContextName: "default",
	}, &buf); err != nil {
		return fmt.Errorf("failed to generate controller manager kubeconfig: %w", err)
	}

	k8sSecrets.ControllerManagerKubeconfig = buf.String()

	buf.Reset()

	if err = kubeconfig.Generate(&kubeconfig.GenerateInput{
		ClusterName: k8sRoot.Name,

		CA:                  k8sRoot.CA,
		CertificateLifetime: KubernetesCertificateValidityDuration,

		CommonName:   constants.KubernetesSchedulerOrganization,
		Organization: constants.KubernetesSchedulerOrganization,

		Endpoint:    k8sRoot.LocalEndpoint.String(),
		Username:    constants.KubernetesSchedulerOrganization,
		ContextName: "default",
	}, &buf); err != nil {
		return fmt.Errorf("failed to generate scheduler kubeconfig: %w", err)
	}

	k8sSecrets.SchedulerKubeconfig = buf.String()

	buf.Reset()

	if err = kubeconfig.GenerateAdmin(&generateAdminAdapter{
		k8sRoot:  k8sRoot,
		endpoint: k8sRoot.Endpoint,
	}, &buf); err != nil {
		return fmt.Errorf("failed to generate admin kubeconfig: %w", err)
	}

	k8sSecrets.AdminKubeconfig = buf.String()

	buf.Reset()

	if err = kubeconfig.GenerateAdmin(&generateAdminAdapter{
		k8sRoot:  k8sRoot,
		endpoint: k8sRoot.LocalEndpoint,
	}, &buf); err != nil {
		return fmt.Errorf("failed to generate admin kubeconfig: %w", err)
	}

	k8sSecrets.LocalhostAdminKubeconfig = buf.String()

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

// generateAdminAdapter allows to translate input config into GenerateAdmin input.
type generateAdminAdapter struct {
	k8sRoot  *secrets.KubernetesRootSpec
	endpoint *url.URL
}

func (adapter *generateAdminAdapter) Name() string {
	return adapter.k8sRoot.Name
}

func (adapter *generateAdminAdapter) Endpoint() *url.URL {
	return adapter.endpoint
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
