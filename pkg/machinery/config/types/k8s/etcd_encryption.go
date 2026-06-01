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
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// KubeEtcdEncryptionConfig defines the KubeEtcdEncryptionConfig configuration name.
const KubeEtcdEncryptionConfig = "KubeEtcdEncryptionConfig"

func init() {
	registry.Register(KubeEtcdEncryptionConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeEtcdEncryptionConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sEtcdEncryptionConfig      = &KubeEtcdEncryptionConfigV1Alpha1{}
	_ config.SecretDocument               = &KubeEtcdEncryptionConfigV1Alpha1{}
	_ config.Validator                    = &KubeEtcdEncryptionConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeEtcdEncryptionConfigV1Alpha1{}
)

// KubeEtcdEncryptionConfigV1Alpha1 configures kube-apiserver etcd encryption rules.
//
//	examples:
//	  - value: exampleKubeEtcdEncryptionConfigV1Alpha1()
//	alias: KubeEtcdEncryptionConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeEtcdEncryptionConfig
type KubeEtcdEncryptionConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Kubernetes API server [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).
	//     Key value should be exact contents of the configuration file, excluding the apiVersion and kind fields.
	//   schema:
	//     type: object
	Config meta.Unstructured `yaml:"config"`
}

// NewKubeEtcdEncryptionConfigV1Alpha1 creates a new KubeEtcdEncryptionConfig config document.
func NewKubeEtcdEncryptionConfigV1Alpha1() *KubeEtcdEncryptionConfigV1Alpha1 {
	return &KubeEtcdEncryptionConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeEtcdEncryptionConfig,
		},
	}
}

func exampleKubeEtcdEncryptionConfigV1Alpha1() *KubeEtcdEncryptionConfigV1Alpha1 {
	cfg := NewKubeEtcdEncryptionConfigV1Alpha1()

	cfg.Config.Object = map[string]any{
		"resources": []any{
			map[string]any{
				"providers": []any{
					map[string]any{
						"secretbox": map[string]any{
							"keys": []any{
								map[string]any{
									"name":   "key2",
									"secret": "M-EXAMPLE-SECRET-DO-NOT-USE-w=",
								},
							},
						},
					},
					map[string]any{
						"identity": map[string]any{},
					},
				},
				"resources": []any{
					"secrets",
				},
			},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeEtcdEncryptionConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *KubeEtcdEncryptionConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if len(s.Config.Object) == 0 {
		errs = errors.Join(errs, errors.New("etcd encryption config is required"))
	}

	return warnings, errs
}

func redactEncryptionConfigObject(o map[string]any, replacement string) {
	// see https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/,
	// the actual secret data is stored in the "secret" field of the config, so we need to replace it with the replacement value.
	for k, v := range o {
		if k == "secret" {
			o[k] = replacement

			continue
		}

		switch v := v.(type) {
		case map[string]any:
			redactEncryptionConfigObject(v, replacement)
		case []any:
			for i := range v {
				if m, ok := v[i].(map[string]any); ok {
					redactEncryptionConfigObject(m, replacement)
				}
			}
		}
	}
}

// Redact implements config.SecretDocument interface.
func (s *KubeEtcdEncryptionConfigV1Alpha1) Redact(replacement string) {
	redactEncryptionConfigObject(s.Config.Object, replacement)
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeEtcdEncryptionConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.Cluster().SecretboxEncryptionSecret() != "" || v1alpha1Cfg.Cluster().AESCBCEncryptionSecret() != "" {
		return errors.New("etcd encryption config is already set in v1alpha1 config")
	}

	return nil
}

// EtcdEncryptionConfig returns the etcd encryption config as a map[string]any.
func (s *KubeEtcdEncryptionConfigV1Alpha1) EtcdEncryptionConfig() map[string]any {
	return s.Config.Object
}
