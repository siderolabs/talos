// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kubeletv1config "k8s.io/kubelet/config/v1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/kubeletstate"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// ServiceManager is the interface to the v1alpha1 services subsystems.
type ServiceManager interface {
	IsRunning(id string) (system.Service, bool, error)
	Load(services ...system.Service) []string
	Stop(ctx context.Context, serviceIDs ...string) (err error)
	Start(serviceIDs ...string) error
}

// KubeletServiceController renders kubelet configuration files and controls kubelet service lifecycle.
type KubeletServiceController struct {
	V1Alpha1Services ServiceManager
}

// Name implements controller.Controller interface.
func (ctrl *KubeletServiceController) Name() string {
	return "k8s.KubeletServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: files.NamespaceName,
			Type:      files.EtcFileStatusType,
			ID:        optional.Some("machine-id"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubeletSpecType,
			ID:        optional.Some(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubeletType,
			ID:        optional.Some(secrets.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			ID:        optional.Some(ctrl.volumeMountRequestID(constants.KubeletDataVolumeID)),
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubeletServiceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// volumeMountRequestID returns the ID of the volume mount request (and the matching volume mount status) for the volume.
func (ctrl *KubeletServiceController) volumeMountRequestID(volumeID string) resource.ID {
	return ctrl.Name() + "-" + volumeID
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *KubeletServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for the machine-id to be generated
		if _, err := r.Get(ctx, resource.NewMetadata(files.NamespaceName, files.EtcFileStatusType, "machine-id", resource.VersionUndefined)); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting etc file status: %w", err)
		}

		cfg, err := safe.ReaderGetByID[*k8s.KubeletSpec](ctx, r, k8s.KubeletID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgSpec := cfg.TypedSpec()

		// kubelet is enabled, so request the KUBELET volume to be mounted (with its parent volume, as the mount
		// controller doesn't mount parents on its own): the controller inspects and cleans up the kubelet state
		// before starting kubelet, so the volume should stay mounted while the controller is active
		for _, volumeID := range []string{"/var/lib", constants.KubeletDataVolumeID} {
			if err = safe.WriterModify(
				ctx, r,
				block.NewVolumeMountRequest(block.NamespaceName, ctrl.volumeMountRequestID(volumeID)),
				func(v *block.VolumeMountRequest) error {
					v.TypedSpec().Requester = ctrl.Name()
					v.TypedSpec().VolumeID = volumeID

					return nil
				},
			); err != nil {
				return fmt.Errorf("error creating volume mount request for %q: %w", volumeID, err)
			}
		}

		kubeletMountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, ctrl.volumeMountRequestID(constants.KubeletDataVolumeID))
		if err != nil {
			if state.IsNotFoundError(err) {
				// KUBELET volume is not mounted yet
				continue
			}

			return fmt.Errorf("error getting volume mount status for the KUBELET volume: %w", err)
		}

		switch kubeletMountStatus.Metadata().Phase() {
		case resource.PhaseTearingDown:
			// the KUBELET volume is being unmounted, release it and stop operating until it's mounted again
			if kubeletMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
				if err = r.RemoveFinalizer(ctx, kubeletMountStatus.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer from volume mount status for the KUBELET volume: %w", err)
				}
			}

			continue
		case resource.PhaseRunning:
			if !kubeletMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
				if err = r.AddFinalizer(ctx, kubeletMountStatus.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error adding finalizer to volume mount status for the KUBELET volume: %w", err)
				}

				// the finalizer update triggers another reconcile, so wait for it to avoid restarting kubelet twice
				continue
			}
		}

		secret, err := safe.ReaderGetByID[*secrets.Kubelet](ctx, r, secrets.KubeletID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets: %w", err)
		}

		secretSpec := secret.TypedSpec()

		var kubeletConfiguration kubeletconfig.KubeletConfiguration

		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(cfgSpec.Config, &kubeletConfiguration); err != nil {
			return fmt.Errorf("error converting kubelet configuration from unstructured: %w", err)
		}

		if err = ctrl.writePKI(secretSpec); err != nil {
			return fmt.Errorf("error writing kubelet PKI: %w", err)
		}

		if err = ctrl.writeConfig(&kubeletConfiguration); err != nil {
			return fmt.Errorf("error writing kubelet configuration: %w", err)
		}

		if err = ctrl.writeKubeletCredentialProviderConfig(cfgSpec); err != nil {
			return fmt.Errorf("error writing kubelet credential provider configuration: %w", err)
		}

		_, running, err := ctrl.V1Alpha1Services.IsRunning("kubelet")
		if err != nil {
			ctrl.V1Alpha1Services.Load(&services.Kubelet{})
		}

		if running {
			if err = ctrl.V1Alpha1Services.Stop(ctx, "kubelet"); err != nil {
				return fmt.Errorf("error stopping kubelet service: %w", err)
			}
		}

		if err = ctrl.refreshKubeletCerts(cfgSpec.ExpectedNodename, secretSpec.AcceptedCAs, logger); err != nil {
			return err
		}

		if err = ctrl.cleanupResourceManagerState(kubeletMountStatus.TypedSpec().Target, &kubeletConfiguration, logger); err != nil {
			return err
		}

		if err = ctrl.refreshSelfServingCert(); err != nil {
			return err
		}

		if err = ctrl.updateKubeconfig(secretSpec.Endpoint, secretSpec.EndpointTLSServerName, secretSpec.AcceptedCAs, logger); err != nil {
			return err
		}

		if err = ctrl.V1Alpha1Services.Start("kubelet"); err != nil {
			return fmt.Errorf("error starting kubelet service: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

// cleanupResourceManagerState removes the kubelet resource manager (CPU manager, memory manager) state files
// which kubelet would refuse to load with the current configuration.
//
// Kubelet validates the persisted state against the configuration and the machine topology on startup, and fails
// if they don't match (e.g. the policy or the set of reserved CPUs changed), so the state is validated the same
// way before kubelet is started.
func (ctrl *KubeletServiceController) cleanupResourceManagerState(kubeletStateDir string, kubeletConfiguration *kubeletconfig.KubeletConfiguration, logger *zap.Logger) error {
	machine, err := kubeletstate.DiscoverMachine()
	if err != nil {
		return fmt.Errorf("error discovering machine topology: %w", err)
	}

	if err = kubeletstate.Cleanup(kubeletStateDir, kubeletConfiguration, machine, logger); err != nil {
		return fmt.Errorf("error cleaning up kubelet resource manager state: %w", err)
	}

	return nil
}

func (ctrl *KubeletServiceController) writePKI(secretSpec *secrets.KubeletSpec) error {
	acceptedCAs := bytes.Join(xslices.Map(secretSpec.AcceptedCAs, func(ca *talosx509.PEMEncodedCertificate) []byte { return ca.Crt }), nil)

	bootstrapKubeconfig := clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			"local": {
				Server:                   secretSpec.Endpoint.String(),
				TLSServerName:            secretSpec.EndpointTLSServerName,
				CertificateAuthorityData: acceptedCAs,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"kubelet@local": {
				Token: fmt.Sprintf("%s.%s", secretSpec.BootstrapTokenID, secretSpec.BootstrapTokenSecret),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"kubelet@local": {
				Cluster:  "local",
				AuthInfo: "kubelet@local",
			},
		},
		CurrentContext: "kubelet@local",
	}

	marshaledKubeConfig, err := clientcmd.Write(bootstrapKubeconfig)
	if err != nil {
		return fmt.Errorf("error marshaling kubeconfig: %w", err)
	}

	if err := os.WriteFile(constants.KubeletBootstrapKubeconfig, marshaledKubeConfig, 0o600); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(constants.KubernetesCACert), 0o700); err != nil {
		return err
	}

	return os.WriteFile(constants.KubernetesCACert, acceptedCAs, 0o400)
}

func (ctrl *KubeletServiceController) writeConfig(kubeletConfiguration *kubeletconfig.KubeletConfiguration) error {
	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		nil,
		nil,
		json.SerializerOptions{
			Yaml: true,
		},
	)

	var buf bytes.Buffer

	if err := serializer.Encode(kubeletConfiguration, &buf); err != nil {
		return err
	}

	return os.WriteFile("/etc/kubernetes/kubelet.yaml", buf.Bytes(), 0o600)
}

func (ctrl *KubeletServiceController) writeKubeletCredentialProviderConfig(cfgSpec *k8s.KubeletSpecSpec) error {
	if cfgSpec.CredentialProviderConfig == nil {
		return os.RemoveAll(constants.KubeletCredentialProviderConfig)
	}

	var kubeletCredentialProviderConfig kubeletv1config.CredentialProviderConfig

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(cfgSpec.CredentialProviderConfig, &kubeletCredentialProviderConfig); err != nil {
		return fmt.Errorf("error converting kubelet credentialprovider configuration from unstructured: %w", err)
	}

	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		nil,
		nil,
		json.SerializerOptions{
			Yaml: true,
		},
	)

	var buf bytes.Buffer

	if err := serializer.Encode(&kubeletCredentialProviderConfig, &buf); err != nil {
		return err
	}

	return os.WriteFile(constants.KubeletCredentialProviderConfig, buf.Bytes(), 0o600)
}

// updateKubeconfig updates the kubeconfig of kubelet with the given endpoint if it exists.
func (ctrl *KubeletServiceController) updateKubeconfig(newEndpoint *url.URL, newTLSServerName string, acceptedCAs []*talosx509.PEMEncodedCertificate, logger *zap.Logger) error {
	config, err := clientcmd.LoadFromFile(constants.KubeletKubeconfig)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	context := config.Contexts[config.CurrentContext]
	if context == nil {
		// this should never happen, but we can't fix kubeconfig if it is malformed
		logger.Error("kubeconfig is missing current context", zap.String("context", config.CurrentContext))

		return nil
	}

	cluster := config.Clusters[context.Cluster]

	if cluster == nil {
		// this should never happen, but we can't fix kubeconfig if it is malformed
		logger.Error("kubeconfig is missing cluster", zap.String("context", config.CurrentContext), zap.String("cluster", context.Cluster))

		return nil
	}

	cluster.Server = newEndpoint.String()
	cluster.TLSServerName = newTLSServerName
	cluster.CertificateAuthorityData = bytes.Join(xslices.Map(acceptedCAs, func(ca *talosx509.PEMEncodedCertificate) []byte { return ca.Crt }), nil)

	return clientcmd.WriteToFile(*config, constants.KubeletKubeconfig)
}

// refreshKubeletCerts checks if the existing kubelet certificates match the node hostname and expected CA.
// If they don't match, it clears the certificate directory and the removes kubelet's kubeconfig so that
// they can be regenerated next time kubelet is started.
//
//nolint:gocyclo
func (ctrl *KubeletServiceController) refreshKubeletCerts(expectedNodename string, acceptedCAs []*talosx509.PEMEncodedCertificate, logger *zap.Logger) error {
	cert, err := ctrl.readKubeletClientCertificate()
	if err != nil {
		return err
	}

	if cert == nil {
		return nil
	}

	valid := true

	// refresh certs only if we are managing the node name (not overridden by the user)
	if expectedNodename != "" {
		expectedCommonName := fmt.Sprintf("system:node:%s", expectedNodename)

		valid = valid && expectedCommonName == cert.Subject.CommonName

		if !valid {
			logger.Info(
				"kubelet client certificate does not match expected nodename, removing",
				zap.String("expected", expectedCommonName),
				zap.String("actual", cert.Subject.CommonName),
			)
		}
	}

	// check against CAs
	if valid {
		rootCAs := x509.NewCertPool()

		for _, ca := range acceptedCAs {
			if !rootCAs.AppendCertsFromPEM(ca.Crt) {
				return fmt.Errorf("error adding CA to root pool: %w", err)
			}
		}

		_, verifyErr := cert.Verify(x509.VerifyOptions{
			Roots:     rootCAs,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		})

		valid = valid && verifyErr == nil

		if !valid {
			logger.Info("kubelet client certificate does not match any accepted CAs, removing", zap.NamedError("verify_error", verifyErr))
		}
	}

	if valid {
		// certificate looks good, no need to refresh
		return nil
	}

	// remove the pki directory
	err = os.RemoveAll(constants.KubeletPKIDir)
	if err != nil {
		return err
	}

	// clear the kubelet kubeconfig
	err = os.Remove(constants.KubeletKubeconfig)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

// refreshSelfServingCert removes the self-signed serving certificate (if exists) to force the kubelet to renew it.
func (ctrl *KubeletServiceController) refreshSelfServingCert() error {
	for _, filename := range []string{
		"kubelet.crt",
		"kubelet.key",
	} {
		path := filepath.Join(constants.KubeletPKIDir, filename)

		_, err := os.Stat(path)
		if err == nil {
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf("error removing self-signed certificate: %w", err)
			}
		}
	}

	return nil
}

func (ctrl *KubeletServiceController) readKubeletClientCertificate() (*x509.Certificate, error) {
	raw, err := os.ReadFile(filepath.Join(constants.KubeletPKIDir, "kubelet-client-current.pem"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	for {
		block, rest := pem.Decode(raw)
		if block == nil {
			return nil, nil
		}

		raw = rest

		if block.Type != "CERTIFICATE" {
			continue
		}

		var cert *x509.Certificate

		cert, err = x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}

		if !cert.IsCA {
			return cert, nil
		}
	}
}
