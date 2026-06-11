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

// KubeProxyConfig defines the KubeProxyConfig configuration name.
const KubeProxyConfig = "KubeProxyConfig"

func init() {
	registry.Register(KubeProxyConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeProxyConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sProxyConfig               = &KubeProxyConfigV1Alpha1{}
	_ config.Validator                    = &KubeProxyConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeProxyConfigV1Alpha1{}
)

// KubeProxyConfigV1Alpha1 deploys Flannel CNI to the cluster.
//
//	examples:
//	  - value: exampleKubeProxyConfigV1Alpha1()
//	alias: KubeProxyConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeProxyConfig
type KubeProxyConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Enable or disable kube-proxy deployment on cluster bootstrap.
	//
	//     Default is enabled.
	ProxyEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The container image used in the kube-proxy manifest.
	ProxyImage string `yaml:"image,omitempty"`
	//   description: |
	//     Proxy mode of kube-proxy.
	//
	//    The default value is 'nftables'.
	//    It is not recommended to use any other value.
	//  values:
	//    - iptables
	//    - ipvs
	//    - nftables
	ProxyMode string `yaml:"mode,omitempty"`
	//   description: |
	//     Provide configuration for the kube-proxy.
	//
	//     There is no need  to specify kind and apiVersion fields (they will be set automatically),
	//     but the rest of the configuration should be provided as is.
	//
	//     See https://kubernetes.io/docs/reference/config-api/kube-proxy-config.v1alpha1/ for the details of the configuration schema.
	//   schema:
	//     type: object
	ProxyConfig meta.Unstructured `yaml:"config"`
	//   description: |
	//     Extra arguments to supply to kube-proxy.
	//
	//     Please note that kube-proxy is configured with a configuration file,
	//     so most flags have no effect.
	//   schema:
	//     type: object
	//     additionalProperties:
	//       oneOf:
	//         - type: string
	//         - type: array
	//           items:
	//             type: string
	ProxyExtraArgs meta.Args `yaml:"extraArgs,omitempty"`
	//   description: |
	//     Configure the kube-proxy resources.
	//   schema:
	//     type: object
	ProxyResources ResourcesConfig `yaml:"resources,omitempty"`
}

// NewKubeProxyConfigV1Alpha1 creates a new KubeProxyConfig config document.
func NewKubeProxyConfigV1Alpha1() *KubeProxyConfigV1Alpha1 {
	return &KubeProxyConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeProxyConfig,
		},
	}
}

func exampleKubeProxyConfigV1Alpha1() *KubeProxyConfigV1Alpha1 {
	cfg := NewKubeProxyConfigV1Alpha1()
	cfg.ProxyMode = "nftables"
	cfg.ProxyImage = constants.KubeProxyImage + ":" + constants.DefaultKubernetesVersion
	cfg.ProxyConfig = meta.Unstructured{
		Object: map[string]any{
			"bindAddressHardFail": true,
		},
	}
	cfg.ProxyResources = ResourcesConfig{
		Requests: meta.Unstructured{
			Object: map[string]any{
				"cpu":    "100m",
				"memory": "50Mi",
			},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeProxyConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *KubeProxyConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	var options validation.Options

	for _, opt := range opts {
		opt(&options)
	}

	if s.ProxyEnabled != nil && !*s.ProxyEnabled {
		// if the kube-proxy is disabled, other fields are not validated
		return warnings, errs
	}

	if s.ProxyImage == "" {
		errs = errors.Join(errs, errors.New("proxy image cannot be empty"))
	} else if !options.Local {
		if err := compatibility.ValidateKubernetesImageTag(s.ProxyImage); err != nil {
			errs = errors.Join(errs, fmt.Errorf("proxy image is not valid: %w", err))
		}
	}

	switch s.ProxyMode {
	case "", "nftables":
		// default and recommended mode, no warnings
	case "iptables", "ipvs":
		warnings = append(warnings, fmt.Sprintf("proxy mode %q is not recommended, please switch to nftables if possible", s.ProxyMode))
	default:
		errs = errors.Join(errs, fmt.Errorf("invalid proxy mode %q: supported modes are iptables, ipvs and nftables", s.ProxyMode))
	}

	if len(s.ProxyExtraArgs) > 0 {
		warnings = append(warnings, "extra arguments for kube-proxy may not work as expected, please use configuration instead")
	}

	extraErrs := s.ProxyResources.Validate()

	errs = errors.Join(errs, extraErrs)

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeProxyConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ProxyConfig != nil { //nolint:staticcheck // legacy access
		return errors.New("cluster proxy config in v1alpha1 config (.machine.cluster.proxy) can't be used with KubeProxyConfig document, please remove it to avoid conflicts")
	}

	return nil
}

// K8sProxyConfigSignal implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) K8sProxyConfigSignal() {}

// Enabled implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) Enabled() bool {
	if s.ProxyEnabled == nil {
		return true
	}

	return *s.ProxyEnabled
}

// Image implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) Image() string {
	return s.ProxyImage
}

// Mode implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) Mode() string {
	return s.ProxyMode
}

// ExtraArgs implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) ExtraArgs() map[string][]string {
	return s.ProxyExtraArgs.ToMap()
}

// Resources implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) Resources() config.Resources {
	return s.ProxyResources
}

// Config implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) Config() map[string]any {
	return s.ProxyConfig.Object
}

// UseConfigFile implements config.K8sProxyConfig interface.
func (s *KubeProxyConfigV1Alpha1) UseConfigFile() bool {
	return true
}
