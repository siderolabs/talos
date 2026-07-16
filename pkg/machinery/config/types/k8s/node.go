// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/go-pointer"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/labels"
)

//docgen:jsonschema

// KubeNodeConfig defines the KubeNodeConfig configuration name.
const KubeNodeConfig = "KubeNodeConfig"

func init() {
	registry.Register(KubeNodeConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeNodeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sNodeConfig                = &KubeNodeConfigV1Alpha1{}
	_ config.Validator                    = &KubeNodeConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeNodeConfigV1Alpha1{}
)

// KubeNodeConfigV1Alpha1 configures Kubernetes node.
//
//	examples:
//	  - value: exampleKubeNodeConfigV1Alpha1()
//	alias: KubeNodeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeNodeConfig
type KubeNodeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//      The `skipNodeRegistration` is used to run the kubelet without registering with the apiserver.
	//      This runs kubelet as standalone and only runs static pods.
	//      When this is set to true, other fields in this document are ignored.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	SkipNodeRegistrationConfig *bool `yaml:"skipNodeRegistration,omitempty"`
	//   description: |
	//     The `registerWithFQDN` field is used to force kubelet to use the node FQDN for registration.
	//     This is required in clouds like AWS.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	RegisterWithFQDNConfig *bool `yaml:"registerWithFQDN,omitempty"`
	//   description: |
	//     The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.
	//     This field should be set when a node has multiple addresses to choose from.
	NodeIPConfig NodeIPConfig `yaml:"nodeIP"`
	//  description: |
	//    Configures the node labels for the machine.
	//
	//    Note: In the default Kubernetes configuration, worker nodes are restricted to set
	//    labels with some prefixes (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin).
	LabelsConfig map[string]string `yaml:"labels,omitempty"`
	//  description: |
	//    Configures the node annotations for the machine.
	AnnotationsConfig map[string]string `yaml:"annotations,omitempty"`
	//  description: |
	//    Configures the node taints for the machine. Effect is optional.
	//
	//    Note: In the default Kubernetes configuration, worker nodes are not allowed to
	//    modify the taints (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin).
	TaintsConfig map[string]string `yaml:"taints,omitempty"`
}

// NodeIPConfig represents the node IP configuration.
type NodeIPConfig struct {
	//  description: |
	//    The `validSubnets` field configures the networks to pick kubelet node IP from.
	//    For dual stack configuration, there should be two subnets: one for IPv4, another for IPv6.
	//    IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.
	//    Negative subnet matches should be specified last to filter out IPs picked by positive matches.
	//    If not specified, node IP is picked based on cluster podCIDRs: IPv4/IPv6 address or both.
	NodeIPValidSubnets []string `yaml:"validSubnets,omitempty"`
}

// NewKubeNodeConfigV1Alpha1 creates a new KubeNodeConfig config document.
func NewKubeNodeConfigV1Alpha1() *KubeNodeConfigV1Alpha1 {
	return &KubeNodeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeNodeConfig,
		},
	}
}

func exampleKubeNodeConfigV1Alpha1() *KubeNodeConfigV1Alpha1 {
	cfg := NewKubeNodeConfigV1Alpha1()
	cfg.RegisterWithFQDNConfig = new(true)
	cfg.NodeIPConfig = NodeIPConfig{
		NodeIPValidSubnets: []string{
			"10.0.0.0/8",
			"!10.0.0.3/32",
			"fdc7::/16",
		},
	}
	cfg.LabelsConfig = map[string]string{
		"examplelabel": "examplevalue",
	}
	cfg.AnnotationsConfig = map[string]string{
		"customer.io/rack": "r13a25",
	}
	cfg.TaintsConfig = map[string]string{
		"exampletaint": "examplevalue:NoSchedule",
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeNodeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeNodeConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	var options validation.Options

	for _, opt := range opts {
		opt(&options)
	}

	for _, cidr := range s.NodeIPConfig.NodeIPValidSubnets {
		cidr = strings.TrimPrefix(cidr, "!")

		if _, err := sideronet.ParseSubnetOrAddress(cidr); err != nil {
			errs = errors.Join(errs, fmt.Errorf("nodeIP subnet is not valid: %q", cidr))
		}
	}

	if err := labels.Validate(s.LabelsConfig); err != nil {
		errs = errors.Join(errs, fmt.Errorf("invalid node labels: %w", err))
	}

	if err := labels.ValidateAnnotations(s.AnnotationsConfig); err != nil {
		errs = errors.Join(errs, fmt.Errorf("invalid node annotations: %w", err))
	}

	if err := labels.ValidateTaints(s.TaintsConfig); err != nil {
		errs = errors.Join(errs, fmt.Errorf("invalid node taints: %w", err))
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
//
//nolint:gocyclo
func (s *KubeNodeConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.AllowSchedulingOnControlPlanes != nil { //nolint:staticcheck // testing deprecated field
		return errors.New(".cluster.allowSchedulingOnControlPlanes is already set in v1alpha1 config")
	}

	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.AllowSchedulingOnMasters != nil { //nolint:staticcheck // testing deprecated field
		return errors.New(".cluster.allowSchedulingOnMasters is already set in v1alpha1 config")
	}

	if v1alpha1Cfg.MachineConfig != nil {
		if v1alpha1Cfg.MachineConfig.MachineKubelet != nil {
			if v1alpha1Cfg.MachineConfig.MachineKubelet.KubeletSkipNodeRegistration != nil { //nolint:staticcheck // testing deprecated field
				return errors.New(".machine.kubelet.skipNodeRegistration is already set in v1alpha1 config")
			}

			if v1alpha1Cfg.MachineConfig.MachineKubelet.KubeletRegisterWithFQDN != nil { //nolint:staticcheck // testing deprecated field
				return errors.New(".machine.kubelet.registerWithFQDN is already set in v1alpha1 config")
			}

			if v1alpha1Cfg.MachineConfig.MachineKubelet.KubeletNodeIP != nil { //nolint:staticcheck // testing deprecated field
				return errors.New(".machine.kubelet.nodeIP is already set in v1alpha1 config")
			}
		}

		if v1alpha1Cfg.MachineConfig.MachineNodeLabels != nil { //nolint:staticcheck // testing deprecated field
			return errors.New(".machine.nodeLabels is already set in v1alpha1 config")
		}

		if v1alpha1Cfg.MachineConfig.MachineNodeAnnotations != nil { //nolint:staticcheck // testing deprecated field
			return errors.New(".machine.nodeAnnotations is already set in v1alpha1 config")
		}

		if v1alpha1Cfg.MachineConfig.MachineNodeTaints != nil { //nolint:staticcheck // testing deprecated field
			return errors.New(".machine.nodeTaints is already set in v1alpha1 config")
		}
	}

	return nil
}

func (s *KubeNodeConfigV1Alpha1) SkipNodeRegistration() bool {
	return pointer.SafeDeref(s.SkipNodeRegistrationConfig)
}

func (s *KubeNodeConfigV1Alpha1) RegisterWithFQDN() bool {
	return pointer.SafeDeref(s.RegisterWithFQDNConfig)
}

func (s *KubeNodeConfigV1Alpha1) NodeIP() config.K8sNodeIPConfig {
	return s.NodeIPConfig
}

func (c NodeIPConfig) ValidSubnets() []string {
	return c.NodeIPValidSubnets
}

func (s *KubeNodeConfigV1Alpha1) Labels() map[string]string {
	return s.LabelsConfig
}

func (s *KubeNodeConfigV1Alpha1) Annotations() map[string]string {
	return s.AnnotationsConfig
}

func (s *KubeNodeConfigV1Alpha1) Taints() map[string]string {
	return s.TaintsConfig
}
