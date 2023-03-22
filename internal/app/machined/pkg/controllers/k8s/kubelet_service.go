// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"text/template"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
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
			ID:        pointer.To("machine-id"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.MountStatusType,
			ID:        pointer.To(constants.EphemeralPartitionLabel),
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
			ID:        pointer.To(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubeletType,
			ID:        pointer.To(secrets.KubeletID),
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

		cfg, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgSpec := cfg.(*k8s.KubeletSpec).TypedSpec()

		secret, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubeletType, secrets.KubeletID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets: %w", err)
		}

		secretSpec := secret.(*secrets.Kubelet).TypedSpec()

		if err = ctrl.writePKI(secretSpec); err != nil {
			return fmt.Errorf("error writing kubelet PKI: %w", err)
		}

		if err = ctrl.writeConfig(cfgSpec); err != nil {
			return fmt.Errorf("error writing kubelet configuration: %w", err)
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

		// refresh certs only if we are managing the node name (not overridden by the user)
		if cfgSpec.ExpectedNodename != "" {
			err = ctrl.refreshKubeletCerts(cfgSpec.ExpectedNodename)
			if err != nil {
				return err
			}
		}

		err = updateKubeconfig(logger, secretSpec.Endpoint)
		if err != nil {
			return err
		}

		if err = ctrl.V1Alpha1Services.Start("kubelet"); err != nil {
			return fmt.Errorf("error starting kubelet service: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *KubeletServiceController) writePKI(secretSpec *secrets.KubeletSpec) error {
	cfg := struct {
		Server               string
		CACert               string
		BootstrapTokenID     string
		BootstrapTokenSecret string
	}{
		Server:               secretSpec.Endpoint.String(),
		CACert:               base64.StdEncoding.EncodeToString(secretSpec.CA.Crt),
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

	return os.WriteFile(constants.KubernetesCACert, secretSpec.CA.Crt, 0o400)
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
			Yaml:   true,
			Pretty: true,
			Strict: true,
		},
	)

	var buf bytes.Buffer

	if err := serializer.Encode(&kubeletConfiguration, &buf); err != nil {
		return err
	}

	return os.WriteFile("/etc/kubernetes/kubelet.yaml", buf.Bytes(), 0o600)
}

// updateKubeconfig updates the kubeconfig of kubelet with the given endpoint if it exists.
func updateKubeconfig(logger *zap.Logger, newEndpoint *url.URL) error {
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

	if cluster.Server == newEndpoint.String() {
		return nil
	}

	cluster.Server = newEndpoint.String()

	return clientcmd.WriteToFile(*config, constants.KubeletKubeconfig)
}

// refreshKubeletCerts checks if the existing kubelet certificates match the node hostname.
// If they don't match, it clears the certificate directory and the removes kubelet's kubeconfig so that
// they can be regenerated next time kubelet is started.
func (ctrl *KubeletServiceController) refreshKubeletCerts(hostname string) error {
	cert, err := ctrl.readKubeletCertificate()
	if err != nil {
		return err
	}

	if cert == nil {
		return nil
	}

	valid := slices.Contains(cert.DNSNames, func(name string) bool {
		return name == hostname
	})

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

func (ctrl *KubeletServiceController) readKubeletCertificate() (*x509.Certificate, error) {
	raw, err := os.ReadFile(filepath.Join(constants.KubeletPKIDir, "kubelet.crt"))
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
