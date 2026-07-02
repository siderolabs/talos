// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

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

// KubeAPIServerConfig defines the KubeAPIServerConfig configuration name.
const KubeAPIServerConfig = "KubeAPIServerConfig"

func init() {
	registry.Register(KubeAPIServerConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAPIServerConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAPIServerConfig           = &KubeAPIServerConfigV1Alpha1{}
	_ config.Validator                    = &KubeAPIServerConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeAPIServerConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &KubeAPIServerConfigV1Alpha1{}
)

// KubeAPIServerConfigV1Alpha1 configures kube-apiserver controlplane static pod.
//
//	examples:
//	  - value: exampleKubeAPIServerConfigV1Alpha1()
//	alias: KubeAPIServerConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAPIServerConfig
type KubeAPIServerConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The container image used to run the kube-apiserver component.
	//
	//     The image reference should contain the tag, even if it is pinned by digest.
	PodImage string `yaml:"image"`
	//   description: |
	//     Extra command line arguments to supply to the kube-apiserver.
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
	//     The `env` field allows for the addition of environment variables for the kube-apiserver.
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	PodEnv map[string]string `yaml:"env,omitempty"`
	//   description: |
	//     Configure the kube-apiserver resources.
	//   schema:
	//     type: object
	PodResources ResourcesConfig `yaml:"resources,omitempty"`
	//   description: |
	//     The port on which the kube-apiserver will listen for requests.
	//
	//     Default is 6443.
	PodAPIPort *int `yaml:"apiPort,omitempty"`
	//  description: |
	//    Provide extra certificate SANs (hostnames, IPs) to add to the kube-apiserver serving certificate.
	//
	//    Talos automatically adds machine's addresses and hostnames, Kubernetes names, and control plane endpoint
	//    derived SANs to the kube-apiserver serving certificate.
	//    This field allows for adding additional SANs to the serving certificate.
	PodCertExtraSANs []string `yaml:"certExtraSANs,omitempty"`
	//   description: |
	//     Enable or disable startup probes for kube-apiserver.
	//
	//     Default is enabled.
	PodStartupProbes *bool `yaml:"startupProbes,omitempty"`
}

// NewKubeAPIServerConfigV1Alpha1 creates a new KubeAPIServerConfig config document.
func NewKubeAPIServerConfigV1Alpha1() *KubeAPIServerConfigV1Alpha1 {
	return &KubeAPIServerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAPIServerConfig,
		},
	}
}

func exampleKubeAPIServerConfigV1Alpha1() *KubeAPIServerConfigV1Alpha1 {
	cfg := NewKubeAPIServerConfigV1Alpha1()
	cfg.PodImage = constants.KubernetesAPIServerImage + ":v" + constants.DefaultKubernetesVersion
	cfg.PodArgs = meta.Args{
		"feature-gates":                    meta.NewArgValue("ServerSideApply=true", nil),
		"http2-max-streams-per-connection": meta.NewArgValue("32", nil),
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeAPIServerConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeAPIServerConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	var options validation.Options

	for _, opt := range opts {
		opt(&options)
	}

	if s.PodImage == "" {
		errs = errors.Join(errs, errors.New("kube-apiserver image cannot be empty"))
	} else if !options.Local {
		if err := compatibility.ValidateKubernetesImageTag(s.PodImage); err != nil {
			errs = errors.Join(errs, fmt.Errorf("kube-apiserver image is not valid: %w", err))
		}
	}

	extraErrs := s.validateArgs()

	errs = errors.Join(errs, extraErrs)

	extraErrs = s.PodResources.Validate()

	errs = errors.Join(errs, extraErrs)

	return warnings, errs
}

func (s *KubeAPIServerConfigV1Alpha1) validateArgs() error {
	deniedPrefixes := map[string]string{
		"anonymous-auth":         "use KubeAuthenticationConfig",
		"oidc-":                  "use KubeAuthenticationConfig",
		"authentication-config":  "use KubeAuthenticationConfig",
		"authorization-config":   "use KubeAuthorizationConfig",
		"authorization-mode":     "use KubeAuthorizationConfig",
		"authorization-webhook-": "use KubeAuthorizationConfig",
	}

	var errs error

	for _, prefix := range slices.Sorted(maps.Keys(deniedPrefixes)) {
		for arg := range s.PodArgs {
			if strings.HasPrefix(arg, prefix) {
				errs = errors.Join(errs, fmt.Errorf("kube-apiserver extra argument %q is not allowed: %s", arg, deniedPrefixes[prefix]))
			}
		}
	}

	return errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeAPIServerConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil {
		if v1alpha1Cfg.ClusterConfig.APIServerConfig != nil { //nolint:staticcheck // testing deprecated field
			return errors.New("kube-apiserver config is already set in v1alpha1 config (.cluster.apiServer)")
		}

		if v1alpha1Cfg.ClusterConfig.ControlPlane != nil && v1alpha1Cfg.ClusterConfig.ControlPlane.LocalAPIServerPort != 0 { //nolint:staticcheck // testing deprecated field
			return errors.New("kube-apiserver API port is already set in v1alpha1 config (.cluster.controlPlane.localAPIServerPort)")
		}
	}

	return nil
}

// K8sAPIServerConfigSignal implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) K8sAPIServerConfigSignal() {}

// Image implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) Image() string {
	return s.PodImage
}

// ExtraArgs implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) ExtraArgs() map[string][]string {
	return s.PodArgs.ToMap()
}

// Env implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) Env() config.Env {
	return s.PodEnv
}

// Resources implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) Resources() config.Resources {
	return s.PodResources
}

// CertSANs implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) CertSANs() []string {
	return s.PodCertExtraSANs
}

//docgen:nodoc
type volumeMount struct {
	VolumeName      string
	VolumeHostPath  string
	VolumeMountPath string
	VolumeReadOnly  bool
}

func (vm volumeMount) Name() string {
	return vm.VolumeName
}

func (vm volumeMount) HostPath() string {
	return vm.VolumeHostPath
}

func (vm volumeMount) MountPath() string {
	return vm.VolumeMountPath
}

func (vm volumeMount) ReadOnly() bool {
	return vm.VolumeReadOnly
}

// ExtraVolumes implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) ExtraVolumes() []config.VolumeMount {
	// by default, mount trusted CA roots from the host into the container
	return []config.VolumeMount{
		volumeMount{
			VolumeName:      "ca-roots",
			VolumeHostPath:  filepath.Dir(constants.DefaultTrustedCAFile),
			VolumeMountPath: filepath.Dir(constants.DefaultTrustedCAFile),
			VolumeReadOnly:  true,
		},
	}
}

// UseAuthenticationConfig implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) UseAuthenticationConfig() bool {
	return true
}

// StartupProbesEnabled implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) StartupProbesEnabled() bool {
	if s.PodStartupProbes == nil {
		return true
	}

	return *s.PodStartupProbes
}

// APIPort implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) APIPort() int {
	if s.PodAPIPort == nil {
		return constants.DefaultControlPlanePort
	}

	return *s.PodAPIPort
}

// InjectDefaultAuthorizers implements config.K8sAPIServerConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) InjectDefaultAuthorizers() bool {
	return false
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeAPIServerConfigV1Alpha1) ControlplaneOnlyDocument() {}
