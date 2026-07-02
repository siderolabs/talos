// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//docgen:jsonschema

// KubeFlannelCNIConfig defines the KubeFlannelCNIConfig configuration name.
const KubeFlannelCNIConfig = "KubeFlannelCNIConfig"

func init() {
	registry.Register(KubeFlannelCNIConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeFlannelCNIConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sFlannelCNIConfig          = &KubeFlannelCNIConfigV1Alpha1{}
	_ config.Validator                    = &KubeFlannelCNIConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeFlannelCNIConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &KubeFlannelCNIConfigV1Alpha1{}
)

// KubeFlannelCNIConfigV1Alpha1 deploys Flannel CNI to the cluster.
//
//	examples:
//	  - value: exampleKubeFlannelCNIConfigV1Alpha1()
//	alias: KubeFlannelCNIConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeFlannelCNIConfig
type KubeFlannelCNIConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Type of the Flannel backend to use.
	//
	//     See Flannel documentation for supported backend types.
	//     The default value in generated machine configuration is "vxlan".
	FlannelBackendType string `yaml:"backendType"`
	//   description: |
	//     UDP port used by Flannel for encapsulating traffic (if the backend type requires encapsulation).
	//
	//     The default value in generated machine configuration is 4789.
	FlannelBackendPort uint16 `yaml:"backendPort,omitempty"`
	//   description: |
	//     Transport MTU to be used for the pod network.
	//
	//     Flannel will subtract encapsulation overhead from this MTU to calculate
	//     the MTU of the pod interface.
	//     If not set, the default is auto-detection of MTU by Flannel.
	//     If KubeSpan is enabled, and the value is not set, defaults to KubeSpan MTU.
	FlannelBackendMTU uint32 `yaml:"backendMTU,omitempty"`
	//   description: |
	//     Extra configuration for Flannel backend.
	//
	//     The content of this field depends on the backend type used.
	//     The value of this field will be patched into Flannel configuration 'Backend' section as-is.
	//   schema:
	//     type: object
	FlannelBackendExtraConfig meta.Unstructured `yaml:"backendExtraConfig,omitempty"`
	//   description: |
	//     Resources configuration for Flannel main container.
	//   schema:
	//     type: object
	FlannelResources ResourcesConfig `yaml:"resources,omitempty"`
	//   description: |
	//     Extra arguments for 'flanneld'.
	//   examples:
	//     - value: >
	//         []string{"--iface-can-reach=192.168.1.1"}
	FlannelExtraArgs []string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     Deploys kube-network-policies along with Flannel.
	//
	//     This enables Kubernetes Network Policies support in the cluster.
	FlannelKubeNetworkPoliciesEnabled *bool `yaml:"kubeNetworkPoliciesEnabled,omitempty"`
}

// NewKubeFlannelCNIConfigV1Alpha1 creates a new KubeFlannelCNIConfig config document.
func NewKubeFlannelCNIConfigV1Alpha1() *KubeFlannelCNIConfigV1Alpha1 {
	return &KubeFlannelCNIConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeFlannelCNIConfig,
		},
	}
}

func exampleKubeFlannelCNIConfigV1Alpha1() *KubeFlannelCNIConfigV1Alpha1 {
	cfg := NewKubeFlannelCNIConfigV1Alpha1()
	cfg.FlannelBackendType = constants.FlannelDefaultBackend
	cfg.FlannelBackendPort = constants.FlannelDefaultBackendPort
	cfg.FlannelBackendMTU = 1420
	cfg.FlannelExtraArgs = []string{"--iface-can-reach=192.168.1.1"}
	cfg.FlannelKubeNetworkPoliciesEnabled = new(true)

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeFlannelCNIConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *KubeFlannelCNIConfigV1Alpha1) Validate(_ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.FlannelBackendType == "" {
		errs = errors.Join(errs, errors.New("flannel backend type must be specified"))
	}

	extraErrs := s.FlannelResources.Validate()

	errs = errors.Join(errs, extraErrs)

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeFlannelCNIConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ClusterNetwork != nil { //nolint:staticcheck // legacy access
		return errors.New("cluster network config in v1alpha1 config (.machine.cluster.network) can't be used with KubeFlannelCNIConfig document, please remove it to avoid conflicts")
	}

	return nil
}

// BackendType implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) BackendType() string {
	return s.FlannelBackendType
}

// BackendPort implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) BackendPort() optional.Optional[uint16] {
	if s.FlannelBackendPort == 0 {
		return optional.None[uint16]()
	}

	return optional.Some(s.FlannelBackendPort)
}

// BackendMTU implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) BackendMTU() optional.Optional[uint32] {
	if s.FlannelBackendMTU == 0 {
		return optional.None[uint32]()
	}

	return optional.Some(s.FlannelBackendMTU)
}

// BackendExtraConfig implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) BackendExtraConfig() map[string]any {
	return s.FlannelBackendExtraConfig.Object
}

// Resources implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) Resources() config.Resources {
	return s.FlannelResources
}

// ExtraArgs implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) ExtraArgs() []string {
	return s.FlannelExtraArgs
}

// KubeNetworkPoliciesEnabled implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) KubeNetworkPoliciesEnabled() bool {
	return pointer.SafeDeref(s.FlannelKubeNetworkPoliciesEnabled)
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) ControlplaneOnlyDocument() {}
