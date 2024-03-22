// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	stdjson "encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"text/template"

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
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
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
	V1Alpha1Mode     runtimetalos.Mode
}

// Name implements controller.Controller interface.
func (ctrl *KubeletServiceController) Name() string {
	return "k8s.KubeletServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletServiceController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KubeletServiceController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *KubeletServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// initially, wait for the machine-id to be generated and /var to be mounted
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: files.NamespaceName,
			Type:      files.EtcFileStatusType,
			ID:        optional.Some("machine-id"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.MountStatusType,
			ID:        optional.Some(constants.EphemeralPartitionLabel),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		_, err := r.Get(ctx, resource.NewMetadata(files.NamespaceName, files.EtcFileStatusType, "machine-id", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting etc file status: %w", err)
		}

		_, err = r.Get(ctx, resource.NewMetadata(runtimeres.NamespaceName, runtimeres.MountStatusType, constants.EphemeralPartitionLabel, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				// in container mode EPHEMERAL is always mounted
				if ctrl.V1Alpha1Mode != runtimetalos.ModeContainer {
					// wait for the EPHEMERAL to be mounted
					continue
				}
			} else {
				return fmt.Errorf("error getting ephemeral mount status: %w", err)
			}
		}

		break
	}

	// normal reconcile loop
	if err := r.UpdateInputs([]controller.Input{
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
	}); err != nil {
		return err
	}

	r.QueueReconcile()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*k8s.KubeletSpec](ctx, r, k8s.KubeletID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgSpec := cfg.TypedSpec()

		secret, err := safe.ReaderGetByID[*secrets.Kubelet](ctx, r, secrets.KubeletID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets: %w", err)
		}

		secretSpec := secret.TypedSpec()

		if err = ctrl.writePKI(secretSpec); err != nil {
			return fmt.Errorf("error writing kubelet PKI: %w", err)
		}

		if err = ctrl.writeConfig(cfgSpec); err != nil {
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

		if err = ctrl.handlePolicyChange(cfgSpec, logger); err != nil {
			return err
		}

		if err = ctrl.refreshSelfServingCert(); err != nil {
			return err
		}

		if err = ctrl.updateKubeconfig(secretSpec.Endpoint, secretSpec.AcceptedCAs, logger); err != nil {
			return err
		}

		if err = ctrl.V1Alpha1Services.Start("kubelet"); err != nil {
			return fmt.Errorf("error starting kubelet service: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

// handlePolicyChange handles the cpuManagerPolicy change.
func (ctrl *KubeletServiceController) handlePolicyChange(cfgSpec *k8s.KubeletSpecSpec, logger *zap.Logger) error {
	const managerFilename = "/var/lib/kubelet/cpu_manager_state"

	oldPolicy, err := loadPolicyFromFile(managerFilename)

	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil // no cpu_manager_state file, nothing to do
	case err != nil:
		return fmt.Errorf("error loading cpu_manager_state file: %w", err)
	}

	policy, err := getFromMap[string](cfgSpec.Config, "cpuManagerPolicy")
	if err != nil {
		return err
	}

	newPolicy := policy.ValueOrZero()
	if equalPolicy(oldPolicy, newPolicy) {
		return nil
	}

	logger.Info("cpuManagerPolicy changed", zap.String("old", oldPolicy), zap.String("new", newPolicy))

	err = os.Remove(managerFilename)
	if err != nil {
		return fmt.Errorf("error removing cpu_manager_state file: %w", err)
	}

	return nil
}

func loadPolicyFromFile(filename string) (string, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	cpuManagerState := struct {
		Policy string `json:"policyName"`
	}{}

	if err = stdjson.Unmarshal(raw, &cpuManagerState); err != nil {
		return "", err
	}

	return cpuManagerState.Policy, nil
}

func equalPolicy(current, newOne string) bool {
	if current == "none" {
		current = ""
	}

	if newOne == "none" {
		newOne = ""
	}

	return current == newOne
}

func getFromMap[T any](m map[string]any, key string) (optional.Optional[T], error) {
	var zero optional.Optional[T]

	res, ok := m[key]
	if !ok {
		return zero, nil
	}

	if res, ok := res.(T); ok {
		return optional.Some(res), nil
	}

	return zero, fmt.Errorf("unexpected type for key %q: found %T, expected %T", key, res, *new(T))
}

func (ctrl *KubeletServiceController) writePKI(secretSpec *secrets.KubeletSpec) error {
	acceptedCAs := bytes.Join(xslices.Map(secretSpec.AcceptedCAs, func(ca *talosx509.PEMEncodedCertificate) []byte { return ca.Crt }), nil)

	cfg := struct {
		Server               string
		CACert               string
		BootstrapTokenID     string
		BootstrapTokenSecret string
	}{
		Server:               secretSpec.Endpoint.String(),
		CACert:               base64.StdEncoding.EncodeToString(acceptedCAs),
		BootstrapTokenID:     secretSpec.BootstrapTokenID,
		BootstrapTokenSecret: secretSpec.BootstrapTokenSecret,
	}

	templ := template.Must(template.New("tmpl").Parse(string(kubeletKubeConfigTemplate)))

	var buf bytes.Buffer

	if err := templ.Execute(&buf, cfg); err != nil {
		return err
	}

	if err := os.WriteFile(constants.KubeletBootstrapKubeconfig, buf.Bytes(), 0o600); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(constants.KubernetesCACert), 0o700); err != nil {
		return err
	}

	return os.WriteFile(constants.KubernetesCACert, acceptedCAs, 0o400)
}

var kubeletKubeConfigTemplate = []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: {{ .Server }}
    certificate-authority-data: {{ .CACert }}
users:
- name: kubelet
  user:
    token: {{ .BootstrapTokenID }}.{{ .BootstrapTokenSecret }}
contexts:
- context:
    cluster: local
    user: kubelet
`)

func (ctrl *KubeletServiceController) writeConfig(cfgSpec *k8s.KubeletSpecSpec) error {
	var kubeletConfiguration kubeletconfig.KubeletConfiguration

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(cfgSpec.Config, &kubeletConfiguration); err != nil {
		return fmt.Errorf("error converting kubelet configuration from unstructured: %w", err)
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

	if err := serializer.Encode(&kubeletConfiguration, &buf); err != nil {
		return err
	}

	return os.WriteFile("/etc/kubernetes/kubelet.yaml", buf.Bytes(), 0o600)
}

func (ctrl *KubeletServiceController) writeKubeletCredentialProviderConfig(cfgSpec *k8s.KubeletSpecSpec) error {
	if cfgSpec.CredentialProviderConfig == nil {
		return os.RemoveAll(constants.KubeletCredentialProviderConfig)
	}

	var kubeletCredentialProviderConfig kubeletconfig.CredentialProviderConfig

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
func (ctrl *KubeletServiceController) updateKubeconfig(newEndpoint *url.URL, acceptedCAs []*talosx509.PEMEncodedCertificate, logger *zap.Logger) error {
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
			logger.Info("kubelet client certificate does not match expected nodename, removing",
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
