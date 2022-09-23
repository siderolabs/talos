// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// RenderConfigsStaticPodController manages k8s.ConfigsReady and renders configs for the control plane.
type RenderConfigsStaticPodController struct{}

// Name implements controller.Controller interface.
func (ctrl *RenderConfigsStaticPodController) Name() string {
	return "k8s.RenderConfigsStaticPodController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RenderConfigsStaticPodController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.AdmissionControlConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.AuditPolicyConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RenderConfigsStaticPodController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.ConfigStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RenderConfigsStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		admissionRes, err := r.Get(ctx, k8s.NewAdmissionControlConfig().Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting admission config resource: %w", err)
		}

		admissionConfig := admissionRes.(*k8s.AdmissionControlConfig).TypedSpec()

		auditRes, err := r.Get(ctx, k8s.NewAuditPolicyConfig().Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting audit config resource: %w", err)
		}

		auditConfig := auditRes.(*k8s.AuditPolicyConfig).TypedSpec()

		type configFile struct {
			filename string
			f        func() (runtime.Object, error)
		}

		serializer := k8sjson.NewSerializerWithOptions(
			k8sjson.DefaultMetaFactory, nil, nil,
			k8sjson.SerializerOptions{
				Yaml:   true,
				Pretty: true,
				Strict: true,
			},
		)

		for _, pod := range []struct {
			name      string
			directory string
			uid       int
			gid       int
			configs   []configFile
		}{
			{
				name:      "kube-apiserver",
				directory: constants.KubernetesAPIServerConfigDir,
				uid:       constants.KubernetesAPIServerRunUser,
				gid:       constants.KubernetesAPIServerRunGroup,
				configs: []configFile{
					{
						filename: "admission-control-config.yaml",
						f:        admissionControlConfig(admissionConfig),
					},
					{
						filename: "auditpolicy.yaml",
						f:        auditPolicyConfig(auditConfig),
					},
				},
			},
		} {
			if err = os.MkdirAll(pod.directory, 0o755); err != nil {
				return fmt.Errorf("error creating config directory for %q: %w", pod.name, err)
			}

			for _, configFile := range pod.configs {
				var obj runtime.Object

				obj, err = configFile.f()
				if err != nil {
					return fmt.Errorf("error generating configuration %q for %q: %w", configFile.filename, pod.name, err)
				}

				var buf bytes.Buffer

				if err = serializer.Encode(obj, &buf); err != nil {
					return fmt.Errorf("error marshaling configuration %q for %q: %w", configFile.filename, pod.name, err)
				}

				if err = os.WriteFile(filepath.Join(pod.directory, configFile.filename), buf.Bytes(), 0o400); err != nil {
					return fmt.Errorf("error writing configuration %q for %q: %w", configFile.filename, pod.name, err)
				}

				if err = os.Chown(filepath.Join(pod.directory, configFile.filename), pod.uid, pod.gid); err != nil {
					return fmt.Errorf("error chowning %q for %q: %w", configFile.filename, pod.name, err)
				}
			}
		}

		if err = r.Modify(ctx, k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID), func(r resource.Resource) error {
			r.(*k8s.ConfigStatus).TypedSpec().Ready = true
			r.(*k8s.ConfigStatus).TypedSpec().Version = admissionRes.Metadata().Version().String() + auditRes.Metadata().Version().String()

			return nil
		}); err != nil {
			return err
		}
	}
}

func admissionControlConfig(spec *k8s.AdmissionControlConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg apiserverv1.AdmissionConfiguration

		cfg.APIVersion = apiserverv1.SchemeGroupVersion.String()
		cfg.Kind = "AdmissionConfiguration"
		cfg.Plugins = []apiserverv1.AdmissionPluginConfiguration{}

		for _, plugin := range spec.Config {
			raw, err := json.Marshal(plugin.Configuration)
			if err != nil {
				return nil, fmt.Errorf("error marshaling configuration for plugin %q: %w", plugin.Name, err)
			}

			cfg.Plugins = append(cfg.Plugins,
				apiserverv1.AdmissionPluginConfiguration{
					Name: plugin.Name,
					Configuration: &runtime.Unknown{
						Raw: raw,
					},
				},
			)
		}

		return &cfg, nil
	}
}

func auditPolicyConfig(spec *k8s.AuditPolicyConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg auditv1.Policy

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(spec.Config, &cfg, true); err != nil {
			return nil, fmt.Errorf("error unmarshaling audit policy configuration: %w", err)
		}

		return &cfg, nil
	}
}
