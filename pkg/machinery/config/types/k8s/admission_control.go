// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// KubeAdmissionControlConfig defines the KubeAdmissionControlConfig configuration name.
const KubeAdmissionControlConfig = "KubeAdmissionControlConfig"

func init() {
	registry.Register(KubeAdmissionControlConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAdmissionControlConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAdmissionControlPluginConfig = &KubeAdmissionControlConfigV1Alpha1{}
	_ config.NamedDocument                   = &KubeAdmissionControlConfigV1Alpha1{}
	_ config.Validator                       = &KubeAdmissionControlConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator    = &KubeAdmissionControlConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig       = &KubeAdmissionControlConfigV1Alpha1{}
)

// KubeAdmissionControlConfigV1Alpha1 configures kube-apiserver admission control plugins.
//
//	examples:
//	  - value: exampleKubeAdmissionControlConfigV1Alpha1()
//	alias: KubeAdmissionControlConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAdmissionControlConfig
type KubeAdmissionControlConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Admission control plugin name, should be a valid Kubernetes admission control plugin name.
	MetaName string `yaml:"name"`
	//   description: |
	//     Kubernetes API server [admission control plugins](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).
	//     The value is the literal Kubernetes admission control configuration.
	//   schema:
	//     type: object
	PluginConfig meta.Unstructured `yaml:"configuration"`
}

// NewKubeAdmissionControlConfigV1Alpha1 creates a new KubeAdmissionControlConfig config document.
func NewKubeAdmissionControlConfigV1Alpha1() *KubeAdmissionControlConfigV1Alpha1 {
	return &KubeAdmissionControlConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAdmissionControlConfig,
		},
	}
}

// DefaultPodSecurityAdmissionControlConfig returns a default PodSecurity admission control plugin configuration.
func DefaultPodSecurityAdmissionControlConfig() *KubeAdmissionControlConfigV1Alpha1 {
	cfg := NewKubeAdmissionControlConfigV1Alpha1()
	cfg.MetaName = "PodSecurity"

	cfg.PluginConfig.Object = map[string]any{
		"apiVersion": "pod-security.admission.config.k8s.io/v1alpha1",
		"kind":       "PodSecurityConfiguration",
		"defaults": map[string]any{
			"enforce":         "baseline",
			"enforce-version": "latest",
			"audit":           "restricted",
			"audit-version":   "latest",
			"warn":            "restricted",
			"warn-version":    "latest",
		},
		"exemptions": map[string]any{
			"usernames":      []any{},
			"runtimeClasses": []any{},
			"namespaces":     []any{"kube-system"},
		},
	}

	return cfg
}

func exampleKubeAdmissionControlConfigV1Alpha1() *KubeAdmissionControlConfigV1Alpha1 {
	return DefaultPodSecurityAdmissionControlConfig()
}

// Clone implements config.Document interface.
func (s *KubeAdmissionControlConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeAdmissionControlConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("admission control plugin name is required"))
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeAdmissionControlConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if len(v1alpha1Cfg.K8sAdmissionControlPluginConfigs()) > 0 {
		return errors.New("admission control plugin config is already set in v1alpha1 config")
	}

	return nil
}

// Name implements config.NamedDocument interface.
func (s *KubeAdmissionControlConfigV1Alpha1) Name() string {
	return s.MetaName
}

// K8sAdmissionControlPluginConfigSignal implements config.K8sAdmissionControlPluginConfig interface.
func (s *KubeAdmissionControlConfigV1Alpha1) K8sAdmissionControlPluginConfigSignal() {}

// Configuration implements config.K8sAdmissionControlPluginConfig interface.
func (s *KubeAdmissionControlConfigV1Alpha1) Configuration() map[string]any {
	return s.PluginConfig.Object
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeAdmissionControlConfigV1Alpha1) ControlplaneOnlyDocument() {}
