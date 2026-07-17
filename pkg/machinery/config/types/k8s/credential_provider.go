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

// KubeCredentialProviderConfig defines the KubeCredentialProviderConfig configuration name.
const KubeCredentialProviderConfig = "KubeCredentialProviderConfig"

func init() {
	registry.Register(KubeCredentialProviderConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeCredentialProviderConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sCredentialProviderConfig  = &KubeCredentialProviderConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeCredentialProviderConfigV1Alpha1{}
)

// KubeCredentialProviderConfigV1Alpha1 configures kubelet's credential provider.
//
//	examples:
//	  - value: exampleKubeCredentialProviderConfigV1Alpha1()
//	alias: KubeCredentialProviderConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeCredentialProviderConfig
type KubeCredentialProviderConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Kubelet credential provider configuration (used for image registry authentication).
	//     The value is the literal kubelet's credential provider configuration.
	//   schema:
	//     type: object
	CredentialProviderConfig meta.Unstructured `yaml:"configuration" merge:"replace"`
}

// NewKubeCredentialProviderConfigV1Alpha1 creates a new KubeCredentialProviderConfig config document.
func NewKubeCredentialProviderConfigV1Alpha1() *KubeCredentialProviderConfigV1Alpha1 {
	return &KubeCredentialProviderConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeCredentialProviderConfig,
		},
	}
}

func exampleKubeCredentialProviderConfigV1Alpha1() *KubeCredentialProviderConfigV1Alpha1 {
	cfg := NewKubeCredentialProviderConfigV1Alpha1()
	cfg.CredentialProviderConfig.Object = map[string]any{
		"apiVersion": "kubelet.config.k8s.io/v1",
		"kind":       "CredentialProviderConfig",
		"providers": []any{
			map[string]any{
				"name":       "ecr-credential-provider",
				"apiVersion": "credentialprovider.kubelet.k8s.io/v1",
				"matchImages": []any{
					"*.dkr.ecr.*.amazonaws.com",
					"*.dkr.ecr.*.amazonaws.com.cn",
					"*.dkr.ecr-fips.*.amazonaws.com",
					"*.dkr.ecr.us-iso-east-1.c2s.ic.gov",
					"*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov",
				},
				"defaultCacheDuration": "12h",
			},
		},
	}

	return cfg
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeCredentialProviderConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineKubelet != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("cannot use KubeCredentialProviderConfig with legacy kubelet configuration (.machine.kubelet)")
	}

	return nil
}

// Clone implements config.Document interface.
func (s *KubeCredentialProviderConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// K8sCredentialProviderConfigSignal implements config.K8sCredentialProviderConfig interface.
func (s *KubeCredentialProviderConfigV1Alpha1) K8sCredentialProviderConfigSignal() {}

// Configuration implements config.K8sCredentialProviderConfig interface.
func (s *KubeCredentialProviderConfigV1Alpha1) Configuration() map[string]any {
	return s.CredentialProviderConfig.Object
}
