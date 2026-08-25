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

// MacVLANKind is a macvlan config document kind.
const MacVLANKind = "MacVLANConfig"

func init() {
	registry.Register(MacVLANKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &MacVLANConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkMacVLANConfig = &MacVLANConfigV1Alpha1{}
	_ config.ConflictingDocument  = &MacVLANConfigV1Alpha1{}
	_ config.NamedDocument        = &MacVLANConfigV1Alpha1{}
	_ config.Validator            = &MacVLANConfigV1Alpha1{}
)

// MacVLANConfigV1Alpha1 is a config document to create a MACVLAN link over a parent link.
//
//	examples:
//	  - value: exampleMacVLANConfigV1Alpha1()
//	alias: MacVLANConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/MacVLANConfig
type MacVLANConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the macvlan link (interface) to be created.
	//
	//   examples:
	//    - value: >
	//       "eth0.macvlan"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     MACVLAN mode to use for the link.
	//     If not set, defaults to bridge.
	//
	//     The `source` mode requires a list of source MAC addresses, which is
	//     not supported yet, so it can't be used.
	//
	//   examples:
	//    - value: >
	//       "bridge"
	//   values:
	//     - "private"
	//     - "vepa"
	//     - "bridge"
	//     - "passthru"
	//     - "source"
	MacVLANMode *nethelpers.MacvlanMode `yaml:"mode,omitempty"`
	//   description: |
	//     Name of the parent link (interface) the macvlan link is created on.
	//     Link aliases can be used here as well.
	//
	//   examples:
	//    - value: >
	//       "eth0"
	//   schemaRequired: true
	MacVLANParent string `yaml:"parent,omitempty"`
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

// NewMacVLANConfigV1Alpha1 creates a new MacVLANConfig config document.
func NewMacVLANConfigV1Alpha1(name string) *MacVLANConfigV1Alpha1 {
	return &MacVLANConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       MacVLANKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleMacVLANConfigV1Alpha1() *MacVLANConfigV1Alpha1 {
	cfg := NewMacVLANConfigV1Alpha1("eth0.macvlan")
	cfg.MacVLANParent = "eth0"
	cfg.MacVLANMode = new(nethelpers.MacvlanModeBridge)

	return cfg
}

// Clone implements config.Document interface.
func (s *MacVLANConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *MacVLANConfigV1Alpha1) Name() string {
	return s.MetaName
}

// MacVLANConfig implements NetworkMacVLANConfig interface.
func (s *MacVLANConfigV1Alpha1) MacVLANConfig() {}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *MacVLANConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(MacVLANKind)
}

// Validate implements config.Validator interface.
func (s *MacVLANConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.MacVLANParent == "" {
		errs = errors.Join(errs, errors.New("parent must be specified"))
	}

	if s.MacVLANMode != nil && !s.MacVLANMode.IsAMacvlanMode() {
		errs = errors.Join(errs, fmt.Errorf("invalid macvlan mode %q", s.MacVLANMode))
	}

	if s.MacVLANMode != nil && *s.MacVLANMode == nethelpers.MacvlanModeSource {
		errs = errors.Join(errs, errors.New("macvlan mode source requires a list of MAC addresses, which is not supported yet"))
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}

// Parent implements NetworkMacVLANConfig interface.
func (s *MacVLANConfigV1Alpha1) Parent() string {
	return s.MacVLANParent
}

// Mode implements NetworkMacVLANConfig interface.
func (s *MacVLANConfigV1Alpha1) Mode() optional.Optional[nethelpers.MacvlanMode] {
	if s.MacVLANMode == nil {
		return optional.None[nethelpers.MacvlanMode]()
	}

	return optional.Some(*s.MacVLANMode)
}

// HardwareAddress implements NetworkHardwareAddressConfig interface.
func (s *MacVLANConfigV1Alpha1) HardwareAddress() optional.Optional[nethelpers.HardwareAddr] {
	if len(s.HardwareAddressConfig) == 0 {
		return optional.None[nethelpers.HardwareAddr]()
	}

	return optional.Some(s.HardwareAddressConfig)
}
