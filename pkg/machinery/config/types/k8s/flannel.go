// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
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

	// more validation will be added here eventually

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeFlannelCNIConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ClusterNetwork != nil { //nolint:staticcheck // legacy access
		return errors.New("cluster network config in v1alpha1 config (.machine.cluster.network) can't be used with KubeFlannelCNIConfig document, please remove it to avoid conflicts")
	}

	return nil
}

// ExtraArgs implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) ExtraArgs() []string {
	return s.FlannelExtraArgs
}

// KubeNetworkPoliciesEnabled implements config.K8sFlannelCNIConfig interface.
func (s *KubeFlannelCNIConfigV1Alpha1) KubeNetworkPoliciesEnabled() bool {
	return pointer.SafeDeref(s.FlannelKubeNetworkPoliciesEnabled)
}
