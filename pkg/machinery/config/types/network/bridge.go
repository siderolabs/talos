// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// BridgeKind is a Bridge config document kind.
const BridgeKind = "BridgeConfig"

func init() {
	registry.Register(BridgeKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &BridgeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkBridgeConfig = &BridgeConfigV1Alpha1{}
	_ config.ConflictingDocument = &BridgeConfigV1Alpha1{}
	_ config.NamedDocument       = &BridgeConfigV1Alpha1{}
	_ config.Validator           = &BridgeConfigV1Alpha1{}
)

// BridgeConfigV1Alpha1 is a config document to create a Bridge (link aggregation) over a set of links.
//
//	examples:
//	  - value: exampleBridgeConfigV1Alpha1()
//	alias: BridgeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/BridgeConfig
type BridgeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the bridge link (interface) to be created.
	//
	//   examples:
	//    - value: >
	//       "Bridge.ext"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Override the hardware (MAC) address of the link.
	//
	//   examples:
	//    - value: >
	//       nethelpers.HardwareAddr{0x2e, 0x3c, 0x4d, 0x5e, 0x6f, 0x70}
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f:]+$
	HardwareAddressConfig nethelpers.HardwareAddr `yaml:"hardwareAddr,omitempty"`
	//   description: |
	//     Names of the links (interfaces) to be aggregated.
	//     Link aliases can be used here as well.
	//   examples:
	//    - value: >
	//       []string{"enp1s3", "enp1s2"}
	//   schemaRequired: true
	BridgeLinks []string `yaml:"links,omitempty"`
	//   description: |
	//     Bridge STP (Spanning Tree Protocol) configuration.
	BridgeSTP BridgeSTPConfig `yaml:"stp,omitempty"`
	//   description: |
	//     Bridge VLAN configuration.
	BridgeVLAN BridgeVLANConfig `yaml:"vlan,omitempty"`

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// BridgeSTPConfig is a bridge STP (Spanning Tree Protocol) configuration.
type BridgeSTPConfig struct {
	//   description: |
	//     Enable or disable STP on the bridge.
	//
	//   examples:
	//    - value: true
	BridgeSTPEnabled *bool `yaml:"enabled,omitempty"`
}

// Enabled implements BridgeSTPConfig interface.
func (s BridgeSTPConfig) Enabled() optional.Optional[bool] {
	if s.BridgeSTPEnabled == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.BridgeSTPEnabled)
}

// BridgeVLANConfig is a bridge VLAN configuration.
type BridgeVLANConfig struct {
	//   description: |
	//     Enable or disable VLAN filtering on the bridge.
	//
	//   examples:
	//    - value: true
	BridgeVLANFiltering *bool `yaml:"filtering,omitempty"`
}

// FilteringEnabled implements BridgeVLANConfig interface.
func (s BridgeVLANConfig) FilteringEnabled() optional.Optional[bool] {
	if s.BridgeVLANFiltering == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.BridgeVLANFiltering)
}

// NewBridgeConfigV1Alpha1 creates a new BridgeConfig config document.
func NewBridgeConfigV1Alpha1(name string) *BridgeConfigV1Alpha1 {
	return &BridgeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       BridgeKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleBridgeConfigV1Alpha1() *BridgeConfigV1Alpha1 {
	cfg := NewBridgeConfigV1Alpha1("bridge.3")
	cfg.BridgeLinks = []string{"eno1", "eno2"}
	cfg.BridgeSTP.BridgeSTPEnabled = new(true)

	return cfg
}

// Clone implements config.Document interface.
func (s *BridgeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *BridgeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// BridgeConfig implements NetworkBridgeConfig interface.
func (s *BridgeConfigV1Alpha1) BridgeConfig() {}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *BridgeConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(BridgeKind)
}

// Validate implements config.Validator interface.
func (s *BridgeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}

// Links implements NetworkBridgeConfig interface.
func (s *BridgeConfigV1Alpha1) Links() []string {
	return s.BridgeLinks
}

// HardwareAddress implements NetworkDummyLinkConfig interface.
func (s *BridgeConfigV1Alpha1) HardwareAddress() optional.Optional[nethelpers.HardwareAddr] {
	if len(s.HardwareAddressConfig) == 0 {
		return optional.None[nethelpers.HardwareAddr]()
	}

	return optional.Some(s.HardwareAddressConfig)
}

// STP implements NetworkBridgeConfig interface.
func (s *BridgeConfigV1Alpha1) STP() config.BridgeSTPConfig {
	return s.BridgeSTP
}

// VLAN implements NetworkBridgeConfig interface.
func (s *BridgeConfigV1Alpha1) VLAN() config.BridgeVLANConfig {
	return s.BridgeVLAN
}
