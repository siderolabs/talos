// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"net/netip"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// DummyLinkKind is a DummyLink config document kind.
const DummyLinkKind = "DummyLinkConfig"

func init() {
	registry.Register(DummyLinkKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &DummyLinkConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkDummyLinkConfig = &DummyLinkConfigV1Alpha1{}
	_ config.ConflictingDocument    = &DummyLinkConfigV1Alpha1{}
	_ config.NamedDocument          = &DummyLinkConfigV1Alpha1{}
	_ config.Validator              = &DummyLinkConfigV1Alpha1{}
)

// DummyLinkConfigV1Alpha1 is a config document to create a dummy (virtual) network link.
//
//	examples:
//	  - value: exampleDummyLinkConfigV1Alpha1()
//	alias: DummyLinkConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/DummyLinkConfig
type DummyLinkConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the dummy link (interface).
	//
	//   examples:
	//    - value: >
	//       "dummy1"
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

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// NewDummyLinkConfigV1Alpha1 creates a new DummyLinkConfig config document.
func NewDummyLinkConfigV1Alpha1(name string) *DummyLinkConfigV1Alpha1 {
	return &DummyLinkConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       DummyLinkKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleDummyLinkConfigV1Alpha1() *DummyLinkConfigV1Alpha1 {
	cfg := NewDummyLinkConfigV1Alpha1("dummy1")
	cfg.LinkAddresses = []AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("192.168.1.100/24"),
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *DummyLinkConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *DummyLinkConfigV1Alpha1) Name() string {
	return s.MetaName
}

// DummyLinkConfig implements NetworkDummyLinkConfig interface.
func (s *DummyLinkConfigV1Alpha1) DummyLinkConfig() {}

// HardwareAddress implements NetworkDummyLinkConfig interface.
func (s *DummyLinkConfigV1Alpha1) HardwareAddress() optional.Optional[nethelpers.HardwareAddr] {
	if len(s.HardwareAddressConfig) == 0 {
		return optional.None[nethelpers.HardwareAddr]()
	}

	return optional.Some(s.HardwareAddressConfig)
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *DummyLinkConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(DummyLinkKind)
}

// Validate implements config.Validator interface.
func (s *DummyLinkConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
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
