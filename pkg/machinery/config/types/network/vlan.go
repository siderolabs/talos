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

// VLANKind is a VLAN config document kind.
const VLANKind = "VLANConfig"

func init() {
	registry.Register(VLANKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &VLANConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkVLANConfig   = &VLANConfigV1Alpha1{}
	_ config.ConflictingDocument = &VLANConfigV1Alpha1{}
	_ config.NamedDocument       = &VLANConfigV1Alpha1{}
	_ config.Validator           = &VLANConfigV1Alpha1{}
)

// VLANConfigV1Alpha1 is a config document to create a VLAN (virtual LAN) over a parent link.
//
//	examples:
//	  - value: exampleVLANConfigV1Alpha1()
//	alias: VLANConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VLANConfig
type VLANConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the VLAN link (interface) to be created.
	//
	//   examples:
	//    - value: >
	//       "enp0s3.34"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     VLAN ID to be used for the VLAN link.
	//
	//   examples:
	//    - value: >
	//       34
	//   schemaRequired: true
	VLANIDConfig uint16 `yaml:"vlanID,omitempty"`
	//   description: |
	//     Set the VLAN mode to use.
	//     If not set, defaults to '802.1q'.
	//
	//   examples:
	//    - value: >
	//       "802.1q"
	//   values:
	//     - "802.1q"
	//     - "802.1ad"
	VLANModeConfig *nethelpers.VLANProtocol `yaml:"vlanMode,omitempty"`
	//   description: |
	//     Name of the parent link (interface) on which the VLAN link will be created.
	//     Link aliases can be used here as well.
	//
	//   examples:
	//    - value: >
	//       "enp0s3"
	//   schemaRequired: true
	ParentLinkConfig string `yaml:"parent,omitempty"`

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// NewVLANConfigV1Alpha1 creates a new VLANConfig config document.
func NewVLANConfigV1Alpha1(name string) *VLANConfigV1Alpha1 {
	return &VLANConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VLANKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleVLANConfigV1Alpha1() *VLANConfigV1Alpha1 {
	cfg := NewVLANConfigV1Alpha1("enp0s3.34")
	cfg.VLANIDConfig = 34
	cfg.ParentLinkConfig = "enp0s3"
	cfg.LinkAddresses = []AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("192.168.1.100/24"),
		},
	}
	cfg.LinkRoutes = []RouteConfig{
		{
			RouteDestination: Prefix{netip.MustParsePrefix("192.168.0.0/16")},
			RouteGateway:     Addr{netip.MustParseAddr("192.168.1.1")},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *VLANConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *VLANConfigV1Alpha1) Name() string {
	return s.MetaName
}

// VLANConfig implements NetworkVLANConfig interface.
func (s *VLANConfigV1Alpha1) VLANConfig() {}

// VLANID returns the VLAN ID.
func (s *VLANConfigV1Alpha1) VLANID() uint16 {
	return s.VLANIDConfig
}

// ParentLink returns the parent link name.
func (s *VLANConfigV1Alpha1) ParentLink() string {
	return s.ParentLinkConfig
}

// VLANMode returns the VLAN mode.
func (s *VLANConfigV1Alpha1) VLANMode() optional.Optional[nethelpers.VLANProtocol] {
	if s.VLANModeConfig == nil {
		return optional.None[nethelpers.VLANProtocol]()
	}

	return optional.Some(*s.VLANModeConfig)
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *VLANConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(VLANKind)
}

// Validate implements config.Validator interface.
func (s *VLANConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.VLANIDConfig == 0 || s.VLANIDConfig > 4094 {
		errs = errors.Join(errs, errors.New("vlanID must be specified and between 1 and 4094"))
	}

	if s.ParentLinkConfig == "" {
		errs = errors.Join(errs, errors.New("parent must be specified"))
	}

	switch {
	case s.VLANModeConfig == nil:
		// default is 802.1q
	case *s.VLANModeConfig != nethelpers.VLANProtocol8021Q && *s.VLANModeConfig != nethelpers.VLANProtocol8021AD:
		errs = errors.Join(errs, errors.New("vlanMode must be either '802.1q' or '802.1ad'"))
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}
