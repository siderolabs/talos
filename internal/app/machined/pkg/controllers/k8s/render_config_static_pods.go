// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// kubeconfigExtension is the extension of the webhook kubeconfig files rendered for kube-apiserver.
const kubeconfigExtension = ".kubeconfig"

// authenticationTokenWebhookKubeconfigFilename is the name of the authentication token webhook kubeconfig file.
const authenticationTokenWebhookKubeconfigFilename = "authentication-token-webhook" + kubeconfigExtension

// authorizationWebhookKubeconfigFilename returns the name of the kubeconfig file for the authorization webhook.
func authorizationWebhookKubeconfigFilename(authorizerName string) string {
	return "authorization-webhook-" + authorizerName + kubeconfigExtension
}

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
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.AuthorizationConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.AuthenticationConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SchedulerConfigType,
			ID:        optional.Some(k8s.FinalSchedulerConfigID),
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
//nolint:gocyclo,cyclop
func (ctrl *RenderConfigsStaticPodController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		admissionRes, err := safe.ReaderGetByID[*k8s.AdmissionControlConfig](ctx, r, k8s.AdmissionControlConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting admission config resource: %w", err)
		}

		admissionConfig := admissionRes.TypedSpec()

		auditRes, err := safe.ReaderGetByID[*k8s.AuditPolicyConfig](ctx, r, k8s.AuditPolicyConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting audit config resource: %w", err)
		}

		auditConfig := auditRes.TypedSpec()

		authorizerConfigRes, err := safe.ReaderGetByID[*k8s.AuthorizationConfig](ctx, r, k8s.AuthorizationConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting authorization config resource: %w", err)
		}

		authorizerConfig := authorizerConfigRes.TypedSpec()

		kubeAPIServerVersion := compatibility.VersionFromImageRef(authorizerConfig.Image)

		// authentication config is optional, so we don't return an error if it is not found
		authenticationConfigRes, err := safe.ReaderGetByID[*k8s.AuthenticationConfig](ctx, r, k8s.AuthenticationConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting authentication config resource: %w", err)
		}

		kubeSchedulerRes, err := safe.ReaderGetByID[*k8s.SchedulerConfig](ctx, r, k8s.FinalSchedulerConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting scheduler config resource: %w", err)
		}

		kubeSchedulerConfig := kubeSchedulerRes.TypedSpec()

		type configFile struct {
			filename string
			f        func() (runtime.Object, error)
		}

		apiServerKubeconfigs, err := apiServerWebhookKubeconfigs(authorizerConfig, authenticationConfigRes)
		if err != nil {
			return err
		}

		apiServerConfigFiles := []configFile{
			{
				filename: "admission-control-config.yaml",
				f:        admissionControlConfig(admissionConfig),
			},
			{
				filename: "auditpolicy.yaml",
				f:        auditPolicyConfig(auditConfig),
			},
			{
				filename: "authorization-config.yaml",
				f:        authorizationConfig(authorizerConfig, kubeAPIServerVersion),
			},
			{
				filename: "authentication-config.yaml",
				f:        authenticationConfig(authenticationConfigRes),
			},
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
			name         string
			directory    string
			selinuxLabel string
			uid          int
			gid          int
			configs      []configFile
			// kubeconfigs are webhook kubeconfigs (filename -> contents) to be rendered, if not nil,
			// stale kubeconfigs not in the map are removed.
			kubeconfigs map[string]string
		}{
			{
				name:         "kube-apiserver",
				directory:    constants.KubernetesAPIServerConfigDir,
				selinuxLabel: constants.KubernetesAPIServerConfigDirSELinuxLabel,
				uid:          constants.KubernetesAPIServerRunUser,
				gid:          constants.KubernetesAPIServerRunGroup,
				configs:      apiServerConfigFiles,
				kubeconfigs:  apiServerKubeconfigs,
			},
			{
				name:         "kube-scheduler",
				directory:    constants.KubernetesSchedulerConfigDir,
				selinuxLabel: constants.KubernetesSchedulerConfigDirSELinuxLabel,
				uid:          constants.KubernetesSchedulerRunUser,
				gid:          constants.KubernetesSchedulerRunGroup,
				configs: []configFile{
					{
						filename: "scheduler-config.yaml",
						f:        schedulerConfig(kubeSchedulerConfig),
					},
				},
			},
		} {
			if err = os.MkdirAll(pod.directory, 0o755); err != nil {
				return fmt.Errorf("error creating config directory for %q: %w", pod.name, err)
			}

			if err = selinux.SetLabel(pod.directory, pod.selinuxLabel); err != nil {
				return err
			}

			// kubeconfigs are written first, as kube-apiserver checks that files referenced in the configuration exist
			for _, filename := range slices.Sorted(maps.Keys(pod.kubeconfigs)) {
				if err = writeStaticPodConfigFile(filepath.Join(pod.directory, filename), []byte(pod.kubeconfigs[filename]), pod.uid, pod.gid); err != nil {
					return fmt.Errorf("error writing kubeconfig %q for %q: %w", filename, pod.name, err)
				}
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

				if err = writeStaticPodConfigFile(filepath.Join(pod.directory, configFile.filename), buf.Bytes(), pod.uid, pod.gid); err != nil {
					return fmt.Errorf("error writing configuration %q for %q: %w", configFile.filename, pod.name, err)
				}
			}

			if pod.kubeconfigs != nil {
				if err = removeStaleKubeconfigs(pod.directory, pod.kubeconfigs); err != nil {
					return fmt.Errorf("error removing stale kubeconfigs for %q: %w", pod.name, err)
				}
			}
		}

		if err = safe.WriterModify(ctx, r, k8s.NewConfigStatus(k8s.ControlPlaneNamespaceName, k8s.ConfigStatusStaticPodID), func(r *k8s.ConfigStatus) error {
			r.TypedSpec().Ready = true
			r.TypedSpec().Version = admissionRes.Metadata().Version().String() +
				auditRes.Metadata().Version().String() +
				authorizerConfigRes.Metadata().Version().String()

			if authenticationConfigRes != nil {
				r.TypedSpec().Version += authenticationConfigRes.Metadata().Version().String()
			}

			return nil
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func writeStaticPodConfigFile(path string, contents []byte, uid, gid int) error {
	if err := os.WriteFile(path, contents, 0o400); err != nil {
		return err
	}

	return os.Chown(path, uid, gid)
}

func removeStaleKubeconfigs(directory string, kubeconfigs map[string]string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), kubeconfigExtension) {
			continue
		}

		if _, ok := kubeconfigs[entry.Name()]; ok {
			continue
		}

		if err = os.Remove(filepath.Join(directory, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

// apiServerWebhookKubeconfigs returns the webhook kubeconfigs to be rendered for kube-apiserver.
func apiServerWebhookKubeconfigs(authorizerConfig *k8s.AuthorizationConfigSpec, authenticationConfigRes *k8s.AuthenticationConfig) (map[string]string, error) {
	kubeconfigs := map[string]string{}

	for _, authorizer := range authorizerConfig.Config {
		if authorizer.Kubeconfig == "" {
			continue
		}

		if err := validateWebhookKubeconfig(authorizer.Kubeconfig); err != nil {
			return nil, fmt.Errorf("error validating kubeconfig for authorizer %q: %w", authorizer.Name, err)
		}

		kubeconfigs[authorizationWebhookKubeconfigFilename(authorizer.Name)] = authorizer.Kubeconfig
	}

	if authenticationConfigRes != nil && authenticationConfigRes.TypedSpec().TokenWebhook != nil {
		kubeconfig := authenticationConfigRes.TypedSpec().TokenWebhook.Kubeconfig

		if err := validateWebhookKubeconfig(kubeconfig); err != nil {
			return nil, fmt.Errorf("error validating authentication token webhook kubeconfig: %w", err)
		}

		kubeconfigs[authenticationTokenWebhookKubeconfigFilename] = kubeconfig
	}

	return kubeconfigs, nil
}

// validateWebhookKubeconfig checks that kube-apiserver is able to build a client from the kubeconfig.
func validateWebhookKubeconfig(kubeconfig string) error {
	cfg, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
		return err
	}

	_, err = clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}).ClientConfig()

	return err
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

			cfg.Plugins = append(
				cfg.Plugins,
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

func schedulerConfig(spec *k8s.SchedulerConfigSpec) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		return &unstructured.Unstructured{Object: spec.Config}, nil
	}
}

func authorizationConfig(spec *k8s.AuthorizationConfigSpec, kubeAPIServerVersion compatibility.Version) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		var cfg apiserverv1.AuthorizationConfiguration

		cfg.APIVersion = kubeAPIServerVersion.KubeAPIServerAuthorizationConfigAPIVersion()
		cfg.Kind = "AuthorizationConfiguration"
		cfg.Authorizers = []apiserverv1.AuthorizerConfiguration{}

		for _, authorizer := range spec.Config {
			authorizerConfig := apiserverv1.AuthorizerConfiguration{
				Name: authorizer.Name,
				Type: authorizer.Type,
			}

			if authorizer.Webhook != nil {
				var webhookCfg apiserverv1.WebhookConfiguration

				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(authorizer.Webhook, &webhookCfg); err != nil {
					return nil, fmt.Errorf("error unmarshaling authorizer webhook configuration: %w", err)
				}

				if authorizer.Kubeconfig != "" {
					webhookCfg.ConnectionInfo = apiserverv1.WebhookConnectionInfo{
						Type:           apiserverv1.AuthorizationWebhookConnectionInfoTypeKubeConfigFile,
						KubeConfigFile: new(filepath.Join(constants.KubernetesAPIServerConfigDir, authorizationWebhookKubeconfigFilename(authorizer.Name))),
					}
				}

				authorizerConfig.Webhook = &webhookCfg
			}

			cfg.Authorizers = append(cfg.Authorizers, authorizerConfig)
		}

		return &cfg, nil
	}
}

// authenticationConfig renders the kube-apiserver authentication configuration.
//
// It enables anonymous authentication only for the health endpoints, so unauthenticated probes can reach them
// while every other endpoint keeps rejecting anonymous requests.
func authenticationConfig(authenticationConfigRes *k8s.AuthenticationConfig) func() (runtime.Object, error) {
	return func() (runtime.Object, error) {
		// the resource might exist only to carry the token webhook configuration
		if authenticationConfigRes == nil || len(authenticationConfigRes.TypedSpec().Config) == 0 {
			return &unstructured.Unstructured{Object: map[string]any{}}, nil
		}

		return &unstructured.Unstructured{Object: authenticationConfigRes.TypedSpec().Config}, nil
	}
}
