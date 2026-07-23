// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kubelet"
)

//docgen:jsonschema

// KubeletConfig defines the KubeletConfig configuration name.
const KubeletConfig = "KubeletConfig"

func init() {
	registry.Register(KubeletConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeletConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sKubeletConfig             = &KubeletConfigV1Alpha1{}
	_ config.Validator                    = &KubeletConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeletConfigV1Alpha1{}
)

// KubeletConfigV1Alpha1 configures kubelet component on the node.
//
//	examples:
//	  - value: exampleKubeletConfigV1Alpha1()
//	alias: KubeletConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeletConfig
type KubeletConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The container image used to run the kubelet component.
	//
	//     The image reference should contain the tag, even if it is pinned by digest.
	//   schemaRequired: true
	KubeletImage string `yaml:"image"`
	//   description: |
	//     Provide extra configuration for the kubelet.
	//
	//     There is no need to specify kind and apiVersion fields (they will be set automatically),
	//     but the rest of the configuration should be provided as is.
	//
	//     See https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/ for the details of the configuration schema.
	//   schema:
	//     type: object
	KubeletConfig meta.Unstructured `yaml:"config"`
	//   description: |
	//     Extra command line arguments to supply to the kubelet.
	//
	//     It is preferable to use `config` field to provide configuration overrides.
	//   schema:
	//     type: object
	//     additionalProperties:
	//       oneOf:
	//         - type: string
	//         - type: array
	//           items:
	//             type: string
	KubeletArgs meta.Args `yaml:"extraArgs,omitempty"`
	//   description: |
	//     The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list.
	KubeletClusterDNS []string `yaml:"clusterDNS,omitempty"`
	//  description: |
	//    Enable container runtime default Seccomp profile.
	//  values:
	//    - true
	//    - yes
	//    - false
	//    - no
	KubeletDefaultRuntimeSeccompProfileEnabled *bool `yaml:"defaultRuntimeSeccompProfileEnabled,omitempty"`
}

// NewKubeletConfigV1Alpha1 creates a new KubeletConfig config document.
func NewKubeletConfigV1Alpha1() *KubeletConfigV1Alpha1 {
	return &KubeletConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeletConfig,
		},
	}
}

func exampleKubeletConfigV1Alpha1() *KubeletConfigV1Alpha1 {
	cfg := NewKubeletConfigV1Alpha1()
	cfg.KubeletImage = constants.KubeletImage + ":v" + constants.DefaultKubernetesVersion
	cfg.KubeletArgs = meta.Args{
		"feature-gates": meta.NewArgValue("AllBeta=true", nil),
	}
	cfg.KubeletConfig = meta.Unstructured{
		Object: map[string]any{
			"serverTLSBootstrap": true,
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeletConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeletConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	var options validation.Options

	for _, opt := range opts {
		opt(&options)
	}

	if s.KubeletImage == "" {
		errs = errors.Join(errs, errors.New("kubelet image cannot be empty"))
	} else if !options.Local {
		if err := compatibility.ValidateKubernetesImageTag(s.KubeletImage); err != nil {
			errs = errors.Join(errs, fmt.Errorf("kubelet image is not valid: %w", err))
		}
	}

	for _, field := range kubelet.ProtectedConfigurationFields {
		if _, exists := s.KubeletConfig.Object[field]; exists {
			errs = errors.Join(errs, fmt.Errorf("kubelet configuration field %q can't be overridden", field))
		}
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeletConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineKubelet != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("kubelet config is already set in v1alpha1 config (.machine.kubelet)")
	}

	return nil
}

// K8sKubeletConfigSignal implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) K8sKubeletConfigSignal() {}

// Image implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) Image() string {
	return s.KubeletImage
}

// ClusterDNS implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) ClusterDNS() []string {
	return s.KubeletClusterDNS
}

// ExtraArgs implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) ExtraArgs() map[string][]string {
	return s.KubeletArgs.ToMap()
}

// ExtraMounts implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) ExtraMounts() []specs.Mount {
	return nil
}

// ExtraConfig implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) ExtraConfig() map[string]any {
	return s.KubeletConfig.Object
}

// DefaultRuntimeSeccompProfileEnabled implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) DefaultRuntimeSeccompProfileEnabled() bool {
	return pointer.SafeDeref(s.KubeletDefaultRuntimeSeccompProfileEnabled)
}

// DisableManifestsDirectory implements config.K8sKubeletConfig interface.
func (s *KubeletConfigV1Alpha1) DisableManifestsDirectory() bool {
	return true
}
