// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//docgen:jsonschema

// KubeControllerManagerConfig defines the KubeControllerManagerConfig configuration name.
const KubeControllerManagerConfig = "KubeControllerManagerConfig"

func init() {
	registry.Register(KubeControllerManagerConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeControllerManagerConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sControllerManagerConfig   = &KubeControllerManagerConfigV1Alpha1{}
	_ config.Validator                    = &KubeControllerManagerConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeControllerManagerConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &KubeControllerManagerConfigV1Alpha1{}
)

// KubeControllerManagerConfigV1Alpha1 configures kube-controller-manager controlplane static pod.
//
//	examples:
//	  - value: exampleKubeControllerManagerConfigV1Alpha1()
//	alias: KubeControllerManagerConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeControllerManagerConfig
type KubeControllerManagerConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     By default, kube-controller-manager static pod is enabled.
	//     Set to false to disable the kube-controller-manager (assuming it runs on other controlplane node).
	PodEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The container image used to run the kube-controller-manager component.
	//
	//     The image reference should contain the tag, even if it is pinned by digest.
	PodImage string `yaml:"image"`
	//   description: |
	//     Extra command line arguments to supply to the kube-controller-manager.
	//   schema:
	//     type: object
	//     additionalProperties:
	//       oneOf:
	//         - type: string
	//         - type: array
	//           items:
	//             type: string
	PodArgs meta.Args `yaml:"extraArgs,omitempty"`
	//   description: |
	//     The `env` field allows for the addition of environment variables for the kube-controller-manager.
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	PodEnv map[string]string `yaml:"env,omitempty"`
	//   description: |
	//     Configure the kube-controller-manager resources.
	//   schema:
	//     type: object
	PodResources ResourcesConfig `yaml:"resources,omitempty"`
}

// NewKubeControllerManagerConfigV1Alpha1 creates a new KubeControllerManagerConfig config document.
func NewKubeControllerManagerConfigV1Alpha1() *KubeControllerManagerConfigV1Alpha1 {
	return &KubeControllerManagerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeControllerManagerConfig,
		},
	}
}

func exampleKubeControllerManagerConfigV1Alpha1() *KubeControllerManagerConfigV1Alpha1 {
	cfg := NewKubeControllerManagerConfigV1Alpha1()
	cfg.PodImage = constants.KubernetesControllerManagerImage + ":v" + constants.DefaultKubernetesVersion
	cfg.PodArgs = meta.Args{
		"feature-gates": meta.NewArgValue("AllBeta=true", nil),
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeControllerManagerConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeControllerManagerConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	var options validation.Options

	for _, opt := range opts {
		opt(&options)
	}

	if s.PodEnabled != nil && !*s.PodEnabled {
		// if the kube-controller-manager is disabled, other fields are not validated
		return warnings, errs
	}

	if s.PodImage == "" {
		errs = errors.Join(errs, errors.New("kube-controller-manager image cannot be empty"))
	} else if !options.Local {
		if err := compatibility.ValidateKubernetesImageTag(s.PodImage); err != nil {
			errs = errors.Join(errs, fmt.Errorf("kube-controller-manager image is not valid: %w", err))
		}
	}

	extraErrs := s.PodResources.Validate()

	errs = errors.Join(errs, extraErrs)

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeControllerManagerConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ControllerManagerConfig != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("kube-controller-manager config is already set in v1alpha1 config (.cluster.controllerManager)")
	}

	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineControlPlane != nil && // nolint:staticcheck // testing deprecated field
		v1alpha1Cfg.MachineConfig.MachineControlPlane.MachineControllerManager != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("kube-controller-manager config is already set in v1alpha1 config (.machine.controlplane.controllerManager)")
	}

	return nil
}

// K8sControllerManagerConfigSignal implements config.K8sControllerManagerConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) K8sControllerManagerConfigSignal() {}

// Enabled implements config.K8sControllerManagerConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) Enabled() bool {
	if s.PodEnabled == nil {
		return true
	}

	return *s.PodEnabled
}

// Image implements config.K8sControllerManagerConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) Image() string {
	return s.PodImage
}

// ExtraArgs implements config.K8sControllerManagerConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) ExtraArgs() map[string][]string {
	return s.PodArgs.ToMap()
}

// Env implements config.K8sControllerManagerConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) Env() config.Env {
	return s.PodEnv
}

// Resources implements config.K8sControllerManagerConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) Resources() config.Resources {
	return s.PodResources
}

// ExtraVolumes implements config.K8sControllerManagerConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) ExtraVolumes() []config.VolumeMount {
	return nil
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeControllerManagerConfigV1Alpha1) ControlplaneOnlyDocument() {}
