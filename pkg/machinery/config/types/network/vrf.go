// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// VRFKind is a VRF config document kind.
const VRFKind = "VRFConfig"

func init() {
	registry.Register(VRFKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &VRFConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkVRFConfig    = &VRFConfigV1Alpha1{}
	_ config.ConflictingDocument = &VRFConfigV1Alpha1{}
	_ config.NamedDocument       = &VRFConfigV1Alpha1{}
	_ config.Validator           = &VRFConfigV1Alpha1{}
)

// VRFConfigV1Alpha1 is a config document to create a vrf and assign links to it.
//
//	examples:
//	  - value: exampleVRFConfigV1Alpha1()
//	alias: VRFConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VRFConfig
type VRFConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the vrf link (interface) to be created.
	//
	//   examples:
	//    - value: >
	//       "vrf-blue"
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
	//     Names of the links (interfaces) to be assigned to this vrf.
	//     Link aliases can be used here as well.
	//   examples:
	//    - value: >
	//       []string{"enp1s3", "enp1s2"}
	//   schemaRequired: true
	VRFLinks []string `yaml:"links,omitempty"`
	//   description: |
	//     Routing table number to use for this vrf.
	//   examples:
	//    - value: >
	//       10
	//   schemaRequired: true
	//   schema:
	//     type: string
	VRFTable nethelpers.RoutingTable `yaml:"table"`

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// NewVRFConfigV1Alpha1 creates a new VRFConfig config document.
func NewVRFConfigV1Alpha1(name string) *VRFConfigV1Alpha1 {
	return &VRFConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VRFKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleVRFConfigV1Alpha1() *VRFConfigV1Alpha1 {
	cfg := NewVRFConfigV1Alpha1("vrf-blue")
	cfg.VRFTable = 10
	cfg.VRFLinks = []string{"eno1", "eno2"}

	return cfg
}

// Clone implements config.Document interface.
func (s *VRFConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *VRFConfigV1Alpha1) Name() string {
	return s.MetaName
}

// VRFConfig implements NetworkVRFConfig interface.
func (s *VRFConfigV1Alpha1) VRFConfig() {}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *VRFConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(VRFKind)
}

// Validate implements config.Validator interface.
func (s *VRFConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.VRFTable == nethelpers.TableUnspec || s.VRFTable == nethelpers.TableLocal || s.VRFTable == nethelpers.TableMain || s.VRFTable == nethelpers.TableDefault {
		errs = errors.Join(errs, fmt.Errorf("cannot create vrf with reserved table %s", s.VRFTable))
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}

// Links implements NetworkVRFConfig interface.
func (s *VRFConfigV1Alpha1) Links() []string {
	return s.VRFLinks
}

// Table implements NetworkVRFConfig interface.
func (s *VRFConfigV1Alpha1) Table() nethelpers.RoutingTable {
	return s.VRFTable
}

// HardwareAddress implements NetworkHardwareAddressConfig interface.
func (s *VRFConfigV1Alpha1) HardwareAddress() optional.Optional[nethelpers.HardwareAddr] {
	if len(s.HardwareAddressConfig) == 0 {
		return optional.None[nethelpers.HardwareAddr]()
	}

	return optional.Some(s.HardwareAddressConfig)
}
