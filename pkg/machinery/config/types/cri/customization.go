// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/labels"
)

//docgen:jsonschema

// CRICustomizationConfigKind is the CRICustomizationConfig configuration document kind.
const CRICustomizationConfigKind = "CRICustomizationConfig"

func init() {
	registry.Register(CRICustomizationConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &CRICustomizationConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.CRICustomizationConfig = &CRICustomizationConfigV1Alpha1{}
	_ config.Validator              = &CRICustomizationConfigV1Alpha1{}
)

// CRICustomizationConfigV1Alpha1 configures the CRI containerd instance.
//
//	examples:
//	  - value: exampleCRICustomizationConfigV1Alpha1()
//	alias: CRICustomizationConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/CRICustomizationConfig
type CRICustomizationConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the CRI customization.
	//
	//     Customizations are merged with physical CRI configuration parts in
	//     lexicographical order by name. The legacy
	//     `/etc/cri/conf.d/20-customization.part` machine file is included under
	//     the reserved name `customization`.
	//
	//     Applying, updating, or removing a customization restarts CRI automatically.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     CRI containerd configuration fragment in TOML format.
	CustomizationContent string `yaml:"content"`
}

// NewCRICustomizationConfigV1Alpha1 creates a new CRICustomizationConfig document.
func NewCRICustomizationConfigV1Alpha1(name string) *CRICustomizationConfigV1Alpha1 {
	return &CRICustomizationConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       CRICustomizationConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleCRICustomizationConfigV1Alpha1() *CRICustomizationConfigV1Alpha1 {
	cfg := NewCRICustomizationConfigV1Alpha1("enable-metrics")
	cfg.CustomizationContent = `[metrics]
  address = "0.0.0.0:11234"
`

	return cfg
}

// Clone implements config.Document interface.
func (cfg *CRICustomizationConfigV1Alpha1) Clone() config.Document {
	return cfg.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (cfg *CRICustomizationConfigV1Alpha1) Name() string {
	return cfg.MetaName
}

// Content implements config.CRICustomizationConfig interface.
func (cfg *CRICustomizationConfigV1Alpha1) Content() string {
	return cfg.CustomizationContent
}

// CRICustomizationConfigSignal implements config.CRICustomizationConfig interface.
func (cfg *CRICustomizationConfigV1Alpha1) CRICustomizationConfigSignal() {}

// Validate implements config.Validator interface.
func (cfg *CRICustomizationConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if err := labels.ValidateQualifiedName(cfg.MetaName); err != nil {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid name: %w", err))
	}

	if cfg.MetaName == config.LegacyCRICustomizationConfigName {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name %q is reserved for the legacy CRI customization", cfg.MetaName))
	}

	return nil, validationErrors
}
