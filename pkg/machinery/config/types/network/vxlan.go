// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// VXLANKind is a VXLAN config document kind.
const VXLANKind = "VXLANConfig"

func init() {
	registry.Register(VXLANKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &VXLANConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkVXLANConfig  = &VXLANConfigV1Alpha1{}
	_ config.ConflictingDocument = &VXLANConfigV1Alpha1{}
	_ config.NamedDocument       = &VXLANConfigV1Alpha1{}
	_ config.Validator           = &VXLANConfigV1Alpha1{}
)

// VXLANConfigV1Alpha1 is a config document to create a VXLAN (Virtual eXtensible LAN) link over a parent link.
//
//	examples:
//	  - value: exampleVXLANConfigV1Alpha1()
//	alias: VXLANConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VXLANConfig
type VXLANConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the vxlan link (interface) to be created.
	//
	//   examples:
	//    - value: >
	//       "vxlan900"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     VXLAN network identifier (VNI) to be used for the vxlan link.
	//
	//   examples:
	//    - value: >
	//       100
	//   schemaRequired: true
	VXLANID uint32 `yaml:"id,omitempty"`
	//   description: |
	//     Source IP address (IPv4 or IPv6) to use in outgoing packets for the tunnel endpoint.
	//
	//   examples:
	//    - value: >
	//       "10.255.0.1"
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f.:]+$
	VXLANLocal meta.Addr `yaml:"local,omitempty"`
	//   description: |
	//     Multicast group IP address (IPv4 or IPv6) to join for the tunnel.
	//     Either the group or the local address should be set, not both.
	//
	//   examples:
	//    - value: >
	//       "239.1.1.1"
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f.:]+$
	VXLANGroup meta.Addr `yaml:"group,omitempty"`
	//   description: |
	//     Name of the parent link (interface) used as the physical device for the tunnel endpoint.
	//     Link aliases can be used here as well.
	//
	//   examples:
	//    - value: >
	//       "vtep0"
	//   schemaRequired: true
	VXLANParent string `yaml:"parent,omitempty"`
	//   description: |
	//     Destination UDP port for VXLAN traffic.
	//     If not set, defaults to 4789.
	//
	//   examples:
	//    - value: >
	//       4789
	VXLANPort *uint16 `yaml:"port,omitempty"`
	//   description: |
	//     Enable learning of source link addresses (MAC learning).
	//     If not set, defaults to true.
	//
	//   examples:
	//    - value: >
	//       false
	VXLANLearning *bool `yaml:"learning,omitempty"`
	//   description: |
	//     Override the hardware (MAC) address of the link.
	//
	//   examples:
	//    - value: >
	//       "2e:3c:4d:5e:6f:70"
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f:]+$
	HardwareAddressConfig nethelpers.HardwareAddr `yaml:"hardwareAddr,omitempty"`

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// NewVXLANConfigV1Alpha1 creates a new VXLANConfig config document.
func NewVXLANConfigV1Alpha1(name string) *VXLANConfigV1Alpha1 {
	return &VXLANConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VXLANKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleVXLANConfigV1Alpha1() *VXLANConfigV1Alpha1 {
	cfg := NewVXLANConfigV1Alpha1("vxlan900")
	cfg.VXLANID = 100
	cfg.VXLANLocal = meta.Addr{Addr: netip.MustParseAddr("10.255.0.1")}
	cfg.VXLANParent = "vtep0"
	cfg.VXLANLearning = new(false)

	return cfg
}

// Clone implements config.Document interface.
func (s *VXLANConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *VXLANConfigV1Alpha1) Name() string {
	return s.MetaName
}

// VXLANConfig implements NetworkVXLANConfig interface.
func (s *VXLANConfigV1Alpha1) VXLANConfig() {}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *VXLANConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(VXLANKind)
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *VXLANConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.VXLANID == 0 {
		errs = errors.Join(errs, errors.New("id must be specified"))
	}

	if s.VXLANID > 0xFFFFFF {
		errs = errors.Join(errs, fmt.Errorf("id must not exceed %d (24-bit VNI)", 0xFFFFFF))
	}

	if s.VXLANParent == "" {
		errs = errors.Join(errs, errors.New("parent must be specified"))
	}

	if s.VXLANLocal != (meta.Addr{}) && s.VXLANGroup != (meta.Addr{}) {
		errs = errors.Join(errs, errors.New("only one of local or group can be specified"))
	}

	if s.VXLANLocal != (meta.Addr{}) && s.VXLANLocal.Addr.IsUnspecified() {
		errs = errors.Join(errs, errors.New("local must not be an unspecified address"))
	}

	if s.VXLANGroup != (meta.Addr{}) && s.VXLANGroup.Addr.IsUnspecified() {
		errs = errors.Join(errs, errors.New("group must not be an unspecified address"))
	}

	if s.VXLANPort != nil && *s.VXLANPort == 0 {
		errs = errors.Join(errs, fmt.Errorf("port must not be 0"))
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}

// ID implements NetworkVXLANConfig interface.
func (s *VXLANConfigV1Alpha1) ID() uint32 {
	return s.VXLANID
}

// Local implements NetworkVXLANConfig interface.
func (s *VXLANConfigV1Alpha1) Local() optional.Optional[netip.Addr] {
	if s.VXLANLocal == (meta.Addr{}) {
		return optional.None[netip.Addr]()
	}

	return optional.Some(s.VXLANLocal.Addr)
}

// Group implements NetworkVXLANConfig interface.
func (s *VXLANConfigV1Alpha1) Group() optional.Optional[netip.Addr] {
	if s.VXLANGroup == (meta.Addr{}) {
		return optional.None[netip.Addr]()
	}

	return optional.Some(s.VXLANGroup.Addr)
}

// Parent implements NetworkVXLANConfig interface.
func (s *VXLANConfigV1Alpha1) Parent() string {
	return s.VXLANParent
}

// Port implements NetworkVXLANConfig interface.
func (s *VXLANConfigV1Alpha1) Port() optional.Optional[uint16] {
	if s.VXLANPort == nil {
		return optional.None[uint16]()
	}

	return optional.Some(*s.VXLANPort)
}

// Learning implements NetworkVXLANConfig interface.
func (s *VXLANConfigV1Alpha1) Learning() optional.Optional[bool] {
	if s.VXLANLearning == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.VXLANLearning)
}

// HardwareAddress implements NetworkHardwareAddressConfig interface.
func (s *VXLANConfigV1Alpha1) HardwareAddress() optional.Optional[nethelpers.HardwareAddr] {
	if len(s.HardwareAddressConfig) == 0 {
		return optional.None[nethelpers.HardwareAddr]()
	}

	return optional.Some(s.HardwareAddressConfig)
}
