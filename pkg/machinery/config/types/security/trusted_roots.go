// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

//docgen:jsonschema

import (
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// TrustedRootsConfig is a default action config document kind.
const TrustedRootsConfig = "TrustedRootsConfig"

func init() {
	registry.Register(TrustedRootsConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &TrustedRootsConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.TrustedRootsConfig = &TrustedRootsConfigV1Alpha1{}
	_ config.NamedDocument      = &TrustedRootsConfigV1Alpha1{}
)

// TrustedRootsConfigV1Alpha1 allows to configure additional trusted CA roots.
//
//	examples:
//	  - value: exampleTrustedRootsConfigV1Alpha1()
//	alias: TrustedRootsConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/TrustedRootsConfig
type TrustedRootsConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     Name of the config document.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     List of additional trusted certificate authorities (as PEM-encoded certificates).
	//
	//     Multiple certificates can be provided in a single config document, separated by newline characters.
	Certificates string `yaml:"certificates"`
}

// NewTrustedRootsConfigV1Alpha1 creates a new TrustedRootsConfig config document.
func NewTrustedRootsConfigV1Alpha1() *TrustedRootsConfigV1Alpha1 {
	return &TrustedRootsConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       TrustedRootsConfig,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleTrustedRootsConfigV1Alpha1() *TrustedRootsConfigV1Alpha1 {
	cfg := NewTrustedRootsConfigV1Alpha1()
	cfg.MetaName = "my-enterprise-ca"
	cfg.Certificates = `-----BEGIN CERTIFICATE-----
...
-----END CERTIFICATE-----
`

	return cfg
}

// Clone implements config.Document interface.
func (s *TrustedRootsConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *TrustedRootsConfigV1Alpha1) Name() string {
	return s.MetaName
}

// ExtraTrustedRootCertificates implements config.TrustedRootsConfig interface.
func (s *TrustedRootsConfigV1Alpha1) ExtraTrustedRootCertificates() []string {
	// build a header with the config name
	header := "\n" + s.MetaName + ":\n" + strings.Repeat("=", len(s.MetaName)+1) + "\n"

	return []string{header + s.Certificates}
}
