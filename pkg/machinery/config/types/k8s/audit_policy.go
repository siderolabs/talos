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
)

//docgen:jsonschema

// KubeAuditPolicyConfig defines the KubeAuditPolicyConfig configuration name.
const KubeAuditPolicyConfig = "KubeAuditPolicyConfig"

func init() {
	registry.Register(KubeAuditPolicyConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAuditPolicyConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAuditPolicyConfig         = &KubeAuditPolicyConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeAuditPolicyConfigV1Alpha1{}
)

// KubeAuditPolicyConfigV1Alpha1 configures kube-apiserver audit policy.
//
//	examples:
//	  - value: exampleKubeAuditPolicyConfigV1Alpha1()
//	alias: KubeAuditPolicyConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAuditPolicyConfig
type KubeAuditPolicyConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Kubernetes API server [audit policy](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/) configuration.
	//     The value is the literal Kubernetes audit policy configuration.
	//   schema:
	//     type: object
	AuditConfig meta.Unstructured `yaml:"configuration" merge:"replace"`
}

// NewKubeAuditPolicyConfigV1Alpha1 creates a new KubeAuditPolicyConfig config document.
func NewKubeAuditPolicyConfigV1Alpha1() *KubeAuditPolicyConfigV1Alpha1 {
	return &KubeAuditPolicyConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAuditPolicyConfig,
		},
	}
}

// DefaultAuditPolicyConfig returns a default audit policy configuration.
func DefaultAuditPolicyConfig() *KubeAuditPolicyConfigV1Alpha1 {
	cfg := NewKubeAuditPolicyConfigV1Alpha1()

	cfg.AuditConfig.Object = map[string]any{
		"apiVersion": "audit.k8s.io/v1",
		"kind":       "Policy",
		"rules": []any{
			map[string]any{
				"level": "Metadata",
			},
		},
	}

	return cfg
}

func exampleKubeAuditPolicyConfigV1Alpha1() *KubeAuditPolicyConfigV1Alpha1 {
	return DefaultAuditPolicyConfig()
}

// Clone implements config.Document interface.
func (s *KubeAuditPolicyConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeAuditPolicyConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.APIServerConfig != nil && //nolint:staticcheck // legacy configuration
		len(v1alpha1Cfg.ClusterConfig.APIServerConfig.AuditPolicyConfig.Object) > 0 { //nolint:staticcheck // legacy configuration
		return errors.New("audit policy config is already set in v1alpha1 config")
	}

	return nil
}

// K8sAuditPolicyConfigSignal implements config.K8sAuditPolicyConfig interface.
func (s *KubeAuditPolicyConfigV1Alpha1) K8sAuditPolicyConfigSignal() {}

// Configuration implements config.K8sAdmissionControlPluginConfig interface.
func (s *KubeAuditPolicyConfigV1Alpha1) Configuration() map[string]any {
	return s.AuditConfig.Object
}
