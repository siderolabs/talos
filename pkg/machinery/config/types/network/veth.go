// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"unicode"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// VethKind is a Veth config document kind.
const VethKind = "VethConfig"

func init() {
	registry.Register(VethKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &VethConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkVethConfig   = &VethConfigV1Alpha1{}
	_ config.ConflictingDocument = &VethConfigV1Alpha1{}
	_ config.NamedDocument       = &VethConfigV1Alpha1{}
	_ config.Validator           = &VethConfigV1Alpha1{}
)

// VethConfigV1Alpha1 is a config document to create a virtual Ethernet device pair.
//
//	examples:
//	  - value: exampleVethConfigV1Alpha1()
//	alias: VethConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VethConfig
type VethConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of this end of the veth pair.
	//
	//     This is a literal kernel interface name. Link aliases are not supported here because
	//     the interface is created by this document rather than selected from existing physical links.
	//   examples:
	//    - value: >
	//       "veth0"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Configuration for the peer end of the veth pair.
	//   schemaRequired: true
	VethPeer VethPeerConfig `yaml:"peer"`

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// VethPeerConfig is the configuration for the peer end of a veth pair.
type VethPeerConfig struct {
	//   description: |
	//     Name of the peer end of the veth pair.
	//
	//     This is a literal kernel interface name. Link aliases are not supported here because
	//     the interface is created by this document rather than selected from existing physical links.
	//
	//     Both endpoints are created in the host network namespace. This name can be listed in a
	//     VRFConfig document's `links` field to attach the peer endpoint to that VRF.
	//   examples:
	//    - value: >
	//       "veth-router"
	//   schemaRequired: true
	VethPeerName string `yaml:"name"`

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// NewVethConfigV1Alpha1 creates a new VethConfig config document.
func NewVethConfigV1Alpha1(name, peerName string) *VethConfigV1Alpha1 {
	return &VethConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VethKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
		VethPeer: VethPeerConfig{
			VethPeerName: peerName,
		},
	}
}

func exampleVethConfigV1Alpha1() *VethConfigV1Alpha1 {
	cfg := NewVethConfigV1Alpha1("veth-host", "veth-router")
	cfg.LinkAddresses = []AddressConfig{{AddressAddress: netip.MustParsePrefix("fda1::1/127")}}
	cfg.VethPeer.LinkAddresses = []AddressConfig{{AddressAddress: netip.MustParsePrefix("fda1::/127")}}

	return cfg
}

// Clone implements config.Document interface.
func (s *VethConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *VethConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Peer implements config.NetworkVethConfig interface.
func (s *VethConfigV1Alpha1) Peer() config.NetworkCommonLinkConfig {
	return &vethPeerLinkConfig{
		CommonLinkConfig: &s.VethPeer.CommonLinkConfig,
		name:             s.VethPeer.VethPeerName,
	}
}

// NetworkVethConfigSignal implements config.NetworkVethConfig.
func (s *VethConfigV1Alpha1) NetworkVethConfigSignal() {}

// AdditionalLinkConfigs implements config.NetworkAdditionalLinkConfigs interface.
func (s *VethConfigV1Alpha1) AdditionalLinkConfigs() []config.NetworkCommonLinkConfig {
	return []config.NetworkCommonLinkConfig{s.Peer()}
}

//docgen:nodoc
type vethPeerLinkConfig struct {
	*CommonLinkConfig

	name string
}

func (s *vethPeerLinkConfig) Name() string {
	return s.name
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *VethConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(VethKind)
}

// Validate implements config.Validator interface.
func (s *VethConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	errs = errors.Join(errs, validateVethLinkName("name", s.MetaName))
	errs = errors.Join(errs, validateVethLinkName("peer name", s.VethPeer.VethPeerName))

	if s.MetaName != "" && s.MetaName == s.VethPeer.VethPeerName {
		errs = errors.Join(errs, errors.New("name and peer name must be different"))
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	extraWarnings, extraErrs = s.VethPeer.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}

func validateVethLinkName(field, name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%s must be specified", field)
	case len(name) > 15:
		return fmt.Errorf("%s must not exceed 15 bytes", field)
	case name == "." || name == "..":
		return fmt.Errorf("%s must not be %q", field, name)
	case strings.ContainsAny(name, "/:"):
		return fmt.Errorf("%s must not contain '/' or ':'", field)
	}

	for _, r := range name {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) {
			return fmt.Errorf("%s must not contain whitespace or non-printable characters", field)
		}
	}

	return nil
}
