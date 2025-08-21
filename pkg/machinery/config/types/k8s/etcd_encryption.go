// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

//docgen:jsonschema

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// EtcdEncryptionConfig is a default action config document kind.
const EtcdEncryptionConfig = "EtcdEncryptionConfig"

func init() {
	registry.Register(EtcdEncryptionConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &EtcdEncryptionConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.EtcdEncryptionConfig = &EtcdEncryptionConfigV1Alpha1{}
)

// EtcdEncryptionConfigV1Alpha1 allows to configure etcd encryption.
//
//	examples:
//	  - value: exampleEtcdEncryptionConfigV1Alpha1()
//	alias: EtcdEncryptionConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/EtcdEncryptionConfig
type EtcdEncryptionConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Custom API server etcd encryption configuration document.
	//     https://kubernetes.io/docs/reference/config-api/apiserver-config.v1/
	//
	config string `yaml:"config"`
}

// NewEtcdEncryptionConfigV1Alpha1 creates a new EtcdEncryptionConfig config document.
func NewEtcdEncryptionConfigV1Alpha1() *EtcdEncryptionConfigV1Alpha1 {
	return &EtcdEncryptionConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       EtcdEncryptionConfig,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleEtcdEncryptionConfigV1Alpha1() *EtcdEncryptionConfigV1Alpha1 {
	cfg := NewEtcdEncryptionConfigV1Alpha1()
	cfg.config = `---
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
          keys:
            - name: key1
              secret: <BASE 64 ENCODED SECRET>
      - identity: {} # this fallback allows reading unencrypted secrets;
                     # for example, during initial migration
`

	return cfg
}

// Clone implements config.Document interface.
func (s *EtcdEncryptionConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// ExtraTrustedRootCertificates implements config.EtcdEncryptionConfig interface.
func (s *EtcdEncryptionConfigV1Alpha1) EtcdEncryptionConfig() string {
	return s.config
}
