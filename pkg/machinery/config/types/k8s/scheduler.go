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

// KubeSchedulerConfig defines the KubeSchedulerConfig configuration name.
const KubeSchedulerConfig = "KubeSchedulerConfig"

func init() {
	registry.Register(KubeSchedulerConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeSchedulerConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sSchedulerConfig           = &KubeSchedulerConfigV1Alpha1{}
	_ config.Validator                    = &KubeSchedulerConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeSchedulerConfigV1Alpha1{}
)

// KubeSchedulerConfigV1Alpha1 configures kube-scheduler controlplane static pod.
//
//	examples:
//	  - value: exampleKubeSchedulerConfigV1Alpha1()
//	alias: KubeSchedulerConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeSchedulerConfig
type KubeSchedulerConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     By default, kube-scheduler static pod is enabled.
	//     Set to false to disable the kube-scheduler (assuming it runs on other controlplane node).
	PodEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The container image used to run the kube-scheduler component.
	//
	//     The image reference should contain the tag, even if it is pinned by digest.
	PodImage string `yaml:"image"`
	//   description: |
	//     Provide configuration for the kube-scheduler static pod.
	//
	//     There is no need  to specify kind and apiVersion fields (they will be set automatically),
	//     but the rest of the configuration should be provided as is.
	//
	//     See https://kubernetes.io/docs/reference/scheduling/config/ for the details of the configuration schema.
	//   schema:
	//     type: object
	PodConfig meta.Unstructured `yaml:"config"`
	//   description: |
	//     Extra command line arguments to supply to the kube-scheduler.
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
	PodArgs meta.Args `yaml:"extraArgs,omitempty"`
	//   description: |
	//     The `env` field allows for the addition of environment variables for the kube-scheduler.
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	PodEnv map[string]string `yaml:"env,omitempty"`
	//   description: |
	//     Configure the kube-scheduler resources.
	//   schema:
	//     type: object
	PodResources ResourcesConfig `yaml:"resources,omitempty"`
}

// NewKubeSchedulerConfigV1Alpha1 creates a new KubeSchedulerConfig config document.
func NewKubeSchedulerConfigV1Alpha1() *KubeSchedulerConfigV1Alpha1 {
	return &KubeSchedulerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeSchedulerConfig,
		},
	}
}

func exampleKubeSchedulerConfigV1Alpha1() *KubeSchedulerConfigV1Alpha1 {
	cfg := NewKubeSchedulerConfigV1Alpha1()
	cfg.PodImage = constants.KubernetesSchedulerImage + ":v" + constants.DefaultKubernetesVersion
	cfg.PodArgs = meta.Args{
		"feature-gates": meta.NewArgValue("AllBeta=true", nil),
	}
	cfg.PodConfig = meta.Unstructured{
		Object: map[string]any{
			"profiles": []any{
				map[string]any{
					"plugins": map[string]any{
						"score": map[string]any{
							"disabled": []any{
								map[string]any{
									"name": "PodTopologySpread",
								},
							},
						},
					},
				},
			},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeSchedulerConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeSchedulerConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	var options validation.Options

	for _, opt := range opts {
		opt(&options)
	}

	if s.PodEnabled != nil && !*s.PodEnabled {
		// if the kube-scheduler is disabled, other fields are not validated
		return warnings, errs
	}

	if s.PodImage == "" {
		errs = errors.Join(errs, errors.New("scheduler image cannot be empty"))
	} else if !options.Local {
		if err := compatibility.ValidateKubernetesImageTag(s.PodImage); err != nil {
			errs = errors.Join(errs, fmt.Errorf("scheduler image is not valid: %w", err))
		}
	}

	extraErrs := s.PodResources.Validate()

	errs = errors.Join(errs, extraErrs)

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeSchedulerConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.SchedulerConfig != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("kube-scheduler config is already set in v1alpha1 config (.cluster.scheduler)")
	}

	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineControlPlane != nil && // nolint:staticcheck // testing deprecated field
		v1alpha1Cfg.MachineConfig.MachineControlPlane.MachineScheduler != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("kube-scheduler config is already set in v1alpha1 config (.machine.controlplane.scheduler)")
	}

	return nil
}

// K8sSchedulerConfigSignal implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) K8sSchedulerConfigSignal() {}

// Enabled implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) Enabled() bool {
	if s.PodEnabled == nil {
		return true
	}

	return *s.PodEnabled
}

// Image implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) Image() string {
	return s.PodImage
}

// ExtraArgs implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) ExtraArgs() map[string][]string {
	return s.PodArgs.ToMap()
}

// Env implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) Env() config.Env {
	return s.PodEnv
}

// Resources implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) Resources() config.Resources {
	return s.PodResources
}

// Config implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) Config() map[string]any {
	return s.PodConfig.Object
}

// ExtraVolumes implements config.K8sSchedulerConfig interface.
func (s *KubeSchedulerConfigV1Alpha1) ExtraVolumes() []config.VolumeMount {
	return nil
}
