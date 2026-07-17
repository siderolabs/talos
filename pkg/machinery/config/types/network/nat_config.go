// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// NATRuleConfigKind is a NAT rule config document kind.
const NATRuleConfigKind = "NetworkNATRuleConfig"

func init() {
	registry.Register(NATRuleConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &NATRuleConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkNATRuleConfig   = &NATRuleConfigV1Alpha1{}
	_ config.NetworkNATConfigSignal = &NATRuleConfigV1Alpha1{}
	_ config.NamedDocument          = &NATRuleConfigV1Alpha1{}
	_ config.Validator              = &NATRuleConfigV1Alpha1{}
)

// NATRuleConfigV1Alpha1 is a network NAT rule config document.
//
//	examples:
//	  - value: exampleNATRuleConfigMasquerade()
//	  - value: exampleNATRuleConfigSNAT()
//	  - value: exampleNATRuleConfigDNAT()
//	alias: NetworkNATRuleConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/NetworkNATRuleConfig
type NATRuleConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the config document.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Type is the kind of NAT operation: masquerade, snat, or dnat.
	//     Defaults to masquerade when omitted.
	//   values:
	//    - "masquerade"
	//    - "snat"
	//    - "dnat"
	Type nethelpers.NATType `yaml:"type,omitempty"`
	//   description: |
	//     SourceAddress restricts which source addresses are matched.
	//     Applies to masquerade, snat, and dnat.
	SourceAddress NATSubnetConfig `yaml:"sourceAddress,omitempty"`
	//   description: |
	//     OutputInterface restricts which egress interfaces trigger the rule.
	//     Applies to masquerade and snat.
	OutputInterface NATInterfaceConfig `yaml:"outputInterface,omitempty"`
	//   description: |
	//     SNATAddress is the address to translate the source to.
	//     Required when type is snat.
	//   schema:
	//     type: string
	SNATAddr Addr `yaml:"snatAddress,omitempty"`
	//   description: |
	//     SNATPort is the source port to translate to.
	//     Optional for snat; when zero, the kernel chooses the source port (default behaviour).
	SNATPortNum uint16 `yaml:"snatPort,omitempty"`
	//   description: |
	//     InputInterface restricts which ingress interfaces trigger the rule.
	//     Applies to dnat.
	InputInterface NATInterfaceConfig `yaml:"inputInterface,omitempty"`
	//   description: |
	//     DestinationAddress restricts which destination addresses are matched.
	//     Applies to snat and dnat.
	DestinationAddress NATSubnetConfig `yaml:"destinationAddress,omitempty"`
	//   description: |
	//     DNATAddress is the address to redirect traffic to.
	//     Required when type is dnat.
	//   schema:
	//     type: string
	DNATAddr Addr `yaml:"dnatAddress,omitempty"`
	//   description: |
	//     DNATPort is the port to redirect traffic to.
	//     Optional for dnat; when zero, the original destination port is preserved.
	DNATPortNum uint16 `yaml:"dnatPort,omitempty"`
}

// NATSubnetConfig holds a list of subnets to match.
type NATSubnetConfig struct {
	//   description: |
	//     IncludeSubnets is the list of CIDRs that match.
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+/\d{1,3}$
	IncludeSubnets []netip.Prefix `yaml:"includeSubnets"`
}

// NATInterfaceConfig holds a list of interface names to match.
type NATInterfaceConfig struct {
	//   description: |
	//     InterfaceNames is the list of interface names to match against.
	InterfaceNames []string `yaml:"interfaceNames"`
}

// NewNATRuleConfigV1Alpha1 creates a new NATRuleConfig config document.
func NewNATRuleConfigV1Alpha1() *NATRuleConfigV1Alpha1 {
	return &NATRuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       NATRuleConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleNATRuleConfigMasquerade() *NATRuleConfigV1Alpha1 {
	cfg := NewNATRuleConfigV1Alpha1()
	cfg.MetaName = "masquerade"
	cfg.Type = nethelpers.NATTypeMasquerade
	cfg.SourceAddress = NATSubnetConfig{
		IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	}
	cfg.OutputInterface = NATInterfaceConfig{
		InterfaceNames: []string{"eth0"},
	}

	return cfg
}

func exampleNATRuleConfigSNAT() *NATRuleConfigV1Alpha1 {
	cfg := NewNATRuleConfigV1Alpha1()
	cfg.MetaName = "snat-rule"
	cfg.Type = nethelpers.NATTypeSNAT
	cfg.SourceAddress = NATSubnetConfig{
		IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	}
	cfg.OutputInterface = NATInterfaceConfig{
		InterfaceNames: []string{"eth0"},
	}
	cfg.SNATAddr = Addr{Addr: netip.MustParseAddr("203.0.113.1")}

	return cfg
}

func exampleNATRuleConfigDNAT() *NATRuleConfigV1Alpha1 {
	cfg := NewNATRuleConfigV1Alpha1()
	cfg.MetaName = "dnat-rule"
	cfg.Type = nethelpers.NATTypeDNAT
	cfg.InputInterface = NATInterfaceConfig{
		InterfaceNames: []string{"eth0"},
	}
	cfg.DestinationAddress = NATSubnetConfig{
		IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("203.0.113.1/32")},
	}
	cfg.DNATAddr = Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	cfg.DNATPortNum = 8080

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *NATRuleConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *NATRuleConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *NATRuleConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.MetaName == "" {
		return nil, errors.New("name is required")
	}

	var warnings []string

	switch s.Type {
	case nethelpers.NATTypeMasquerade:
		if s.SNATAddr.IsValid() {
			warnings = append(warnings, "snatAddress has no effect on masquerade rules")
		}

		if s.SNATPortNum != 0 {
			warnings = append(warnings, "snatPort has no effect on masquerade rules")
		}

		if len(s.DestinationAddress.IncludeSubnets) > 0 {
			warnings = append(warnings, "destinationAddress has no effect on masquerade rules")
		}

		if s.DNATAddr.IsValid() {
			warnings = append(warnings, "dnatAddress has no effect on masquerade rules")
		}

		if len(s.InputInterface.InterfaceNames) > 0 {
			warnings = append(warnings, "inputInterface has no effect on masquerade rules (ingress interface is not matched at postrouting)")
		}
	case nethelpers.NATTypeSNAT:
		if !s.SNATAddr.IsValid() {
			return nil, errors.New("snatAddress is required for type snat")
		}

		if s.DNATAddr.IsValid() {
			warnings = append(warnings, "dnatAddress has no effect on snat rules")
		}

		if len(s.InputInterface.InterfaceNames) > 0 {
			warnings = append(warnings, "inputInterface has no effect on snat rules (ingress interface is not matched at postrouting)")
		}
	case nethelpers.NATTypeDNAT:
		if !s.DNATAddr.IsValid() {
			return nil, errors.New("dnatAddress is required for type dnat")
		}

		if len(s.OutputInterface.InterfaceNames) > 0 {
			warnings = append(warnings, "outputInterface has no effect on dnat rules (egress interface is not known at prerouting)")
		}

		if s.SNATAddr.IsValid() {
			warnings = append(warnings, "snatAddress has no effect on dnat rules")
		}

		if s.SNATPortNum != 0 {
			warnings = append(warnings, "snatPort has no effect on dnat rules")
		}
	default:
		return nil, fmt.Errorf("unknown NAT type %q", s.Type)
	}

	for _, pfx := range s.SourceAddress.IncludeSubnets {
		if !pfx.IsValid() {
			return warnings, fmt.Errorf("invalid sourceAddress subnet: %s", pfx)
		}
	}

	for _, pfx := range s.DestinationAddress.IncludeSubnets {
		if !pfx.IsValid() {
			return warnings, fmt.Errorf("invalid destinationAddress subnet: %s", pfx)
		}
	}

	return warnings, nil
}

// NetworkNATConfigSignal implements config.NetworkNATConfigSignal interface.
func (s *NATRuleConfigV1Alpha1) NetworkNATConfigSignal() {}

// NATRules implements config.NetworkNATRuleConfig interface.
func (s *NATRuleConfigV1Alpha1) NATRules() []config.NetworkNATRule {
	return []config.NetworkNATRule{s}
}

// NATType implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) NATType() nethelpers.NATType {
	return s.Type
}

// SourceSubnets implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) SourceSubnets() []netip.Prefix {
	return s.SourceAddress.IncludeSubnets
}

// DestinationSubnets implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) DestinationSubnets() []netip.Prefix {
	return s.DestinationAddress.IncludeSubnets
}

// OutputInterfaces implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) OutputInterfaces() []string {
	return s.OutputInterface.InterfaceNames
}

// InputInterfaces implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) InputInterfaces() []string {
	return s.InputInterface.InterfaceNames
}

// SNATAddress implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) SNATAddress() *netip.Addr {
	if !s.SNATAddr.IsValid() {
		return nil
	}

	addr := s.SNATAddr.Addr

	return &addr
}

// SNATPort implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) SNATPort() uint16 {
	return s.SNATPortNum
}

// DNATAddress implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) DNATAddress() *netip.Addr {
	if !s.DNATAddr.IsValid() {
		return nil
	}

	addr := s.DNATAddr.Addr

	return &addr
}

// DNATPort implements config.NetworkNATRule interface.
func (s *NATRuleConfigV1Alpha1) DNATPort() uint16 {
	return s.DNATPortNum
}
