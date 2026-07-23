// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// UdevRulesConfigKind is a udev rules config document kind.
const UdevRulesConfigKind = "UdevRulesConfig"

func init() {
	registry.Register(UdevRulesConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &UdevRulesConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var _ config.UdevConfig = &UdevRulesConfigV1Alpha1{}

// UdevRulesConfigV1Alpha1 is a udev rules config document.
//
//	examples:
//	  - value: exampleUdevRulesConfigV1Alpha1()
//	alias: UdevRulesConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/UdevRulesConfig
type UdevRulesConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Custom udev rules.
	UdevRules []string `yaml:"rules,omitempty"`
}

// NewUdevRulesConfigV1Alpha1 creates a new udev rules config document.
func NewUdevRulesConfigV1Alpha1() *UdevRulesConfigV1Alpha1 {
	return &UdevRulesConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       UdevRulesConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleUdevRulesConfigV1Alpha1() *UdevRulesConfigV1Alpha1 {
	cfg := NewUdevRulesConfigV1Alpha1()
	cfg.UdevRules = []string{`SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"`}

	return cfg
}

// Clone implements config.Document interface.
func (s *UdevRulesConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Rules implements config.UdevConfig interface.
func (s *UdevRulesConfigV1Alpha1) Rules() []string {
	return s.UdevRules
}
