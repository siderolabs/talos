// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//docgen:jsonschema

// KubeAuthenticationConfig defines the KubeAuthenticationConfig configuration name.
const KubeAuthenticationConfig = "KubeAuthenticationConfig"

func init() {
	registry.Register(KubeAuthenticationConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAuthenticationConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAuthenticationConfig = &KubeAuthenticationConfigV1Alpha1{}
)

// KubeAuthenticationConfigV1Alpha1 configures kube-apiserver authentication.
//
//	examples:
//	  - value: exampleKubeAuthenticationConfigV1Alpha1()
//	alias: KubeAuthenticationConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAuthenticationConfig
type KubeAuthenticationConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Kubernetes API server [authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/) configuration.
	//     The value is the literal Kubernetes authentication configuration.
	//   schema:
	//     type: object
	AuthConfig meta.Unstructured `yaml:"configuration" merge:"replace"`
}

// NewKubeAuthenticationConfigV1Alpha1 creates a new KubeAuthenticationConfig config document.
func NewKubeAuthenticationConfigV1Alpha1() *KubeAuthenticationConfigV1Alpha1 {
	return &KubeAuthenticationConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAuthenticationConfig,
		},
	}
}

// DefaultAuthenticationConfig returns a default authentication configuration.
func DefaultAuthenticationConfig() *KubeAuthenticationConfigV1Alpha1 {
	cfg := NewKubeAuthenticationConfigV1Alpha1()

	cfg.AuthConfig.Object = map[string]any{
		"anonymous": map[string]any{
			"conditions": []any{
				map[string]any{
					"path": "/livez",
				},
				map[string]any{
					"path": "/readyz",
				},
				map[string]any{
					"path": "/healthz",
				},
			},
			"enabled": true,
		},
		"jwt": []any{},
	}

	return cfg
}

func exampleKubeAuthenticationConfigV1Alpha1() *KubeAuthenticationConfigV1Alpha1 {
	return DefaultAuthenticationConfig()
}

// Clone implements config.Document interface.
func (s *KubeAuthenticationConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// K8sAuthenticationConfigSignal implements config.K8sAuthenticationConfig interface.
func (s *KubeAuthenticationConfigV1Alpha1) K8sAuthenticationConfigSignal() {}

// Configuration implements config.K8sAdmissionControlPluginConfig interface.
func (s *KubeAuthenticationConfigV1Alpha1) Configuration() map[string]any {
	return s.AuthConfig.Object
}
