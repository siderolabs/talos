// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/kubeconfig"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
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
			Type:      secrets.KubernetesRootType,
			ID:        optional.Some(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeresource.StatusType,
			ID:        optional.Some(timeresource.StatusID),
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
func (ctrl *KubernetesController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	refreshTicker := time.NewTicker(KubernetesCertificateValidityDuration / 2)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-refreshTicker.C:
		}

		k8sRoot, err := safe.ReaderGet[*secrets.KubernetesRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting root k8s secrets: %w", err)
		}

		// wait for time sync as certs depend on current time
		timeSync, err := safe.ReaderGet[*timeresource.Status](ctx, r, resource.NewMetadata(v1alpha1.NamespaceName, timeresource.StatusType, timeresource.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !timeSync.TypedSpec().Synced {
			continue
		}

		if err = safe.WriterModify(ctx, r, secrets.NewKubernetes(), func(r *secrets.Kubernetes) error {
			return ctrl.updateSecrets(k8sRoot.TypedSpec(), r.TypedSpec())
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *KubernetesController) updateSecrets(k8sRoot *secrets.KubernetesRootSpec, k8sSecrets *secrets.KubernetesCertsSpec) error {
	var buf bytes.Buffer

	if err := kubeconfig.Generate(&kubeconfig.GenerateInput{
		ClusterName: k8sRoot.Name,

		IssuingCA:           k8sRoot.IssuingCA,
		AcceptedCAs:         k8sRoot.AcceptedCAs,
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

	if err := kubeconfig.Generate(&kubeconfig.GenerateInput{
		ClusterName: k8sRoot.Name,

		IssuingCA:           k8sRoot.IssuingCA,
		AcceptedCAs:         k8sRoot.AcceptedCAs,
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

	if err := kubeconfig.GenerateAdmin(&generateAdminAdapter{
		k8sRoot:  k8sRoot,
		endpoint: k8sRoot.Endpoint,
	}, &buf); err != nil {
		return fmt.Errorf("failed to generate admin kubeconfig: %w", err)
	}

	k8sSecrets.AdminKubeconfig = buf.String()

	buf.Reset()

	if err := kubeconfig.GenerateAdmin(&generateAdminAdapter{
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

func (adapter *generateAdminAdapter) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	return adapter.k8sRoot.IssuingCA
}

func (adapter *generateAdminAdapter) AcceptedCAs() []*x509.PEMEncodedCertificate {
	return adapter.k8sRoot.AcceptedCAs
}

func (adapter *generateAdminAdapter) AdminKubeconfig() config.AdminKubeconfig {
	return adapter
}

func (adapter *generateAdminAdapter) CertLifetime() time.Duration {
	// this certificate is not delivered to the user, it's used only internally by control plane components
	return KubernetesCertificateValidityDuration
}

func (adapter *generateAdminAdapter) CommonName() string {
	return constants.KubernetesTalosAdminCertCommonName
}

func (adapter *generateAdminAdapter) CertOrganization() string {
	return constants.KubernetesAdminCertOrganization
}
