// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/files"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
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
	// initially, wait for the cri to be up and for machine-id to be generated
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("cri"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: files.NamespaceName,
			Type:      files.EtcFileStatusType,
			ID:        pointer.ToString("machine-id"),
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

		svc, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "cri", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting service: %w", err)
		}

		if svc.(*v1alpha1.Service).Healthy() && svc.(*v1alpha1.Service).Running() {
			break
		}
	}

	// normal reconcile loop, ignore cri state
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubeletSpecType,
			ID:        pointer.ToString(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubeletType,
			ID:        pointer.ToString(secrets.KubeletID),
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

		if err = ctrl.V1Alpha1Services.Start("kubelet"); err != nil {
			return fmt.Errorf("error starting kubelet service: %w", err)
		}
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

	if err := ioutil.WriteFile(constants.KubeletBootstrapKubeconfig, buf.Bytes(), 0o600); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(constants.KubernetesCACert), 0o700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubernetesCACert, secretSpec.CA.Crt, 0o400); err != nil {
		return err
	}

	return nil
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

	return ioutil.WriteFile("/etc/kubernetes/kubelet.yaml", buf.Bytes(), 0o600)
}
