// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// RuleConfigKind is a rule config document kind.
const RuleConfigKind = "NetworkRuleConfig"

func init() {
	registry.Register(RuleConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &RuleConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkRuleConfigRules  = &RuleConfigV1Alpha1{}
	_ config.NetworkRuleConfigSignal = &RuleConfigV1Alpha1{}
	_ config.NamedDocument           = &RuleConfigV1Alpha1{}
	_ config.Validator               = &RuleConfigV1Alpha1{}
)

// RuleConfigV1Alpha1 is a network firewall rule config document.
//
//docgen:version=v1alpha1
type RuleConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	// Name of the config document.
	MetaName string `yaml:"name" docgen:"{'in':'1.7','required':true}"`
	// The port selector defines which ports and protocols on the host are affected by the rule.
	PortSelector RulePortSelector `yaml:"portSelector" docgen:"{'in':'1.7'}"`
	// Defines which source subnets are allowed to access the host ports/protocols defined by the `portSelector`.
	Ingress IngressConfig `yaml:"ingress" merge:"replace" docgen:"{'in':'1.7'}"`
}

// RulePortSelector is a port selector for the network rule.
type RulePortSelector struct {
	// Defines a list of port ranges or single ports. The port ranges are inclusive, and should not overlap.
	Ports PortRanges `yaml:"ports" merge:"replace" docgen:"{'in':'1.7'}"`
	// Defines traffic protocol (e.g. TCP or UDP).
	Protocol nethelpers.Protocol `yaml:"protocol" docgen:"{'in':'1.7','values':['tcp','udp','icmp','icmpv6']}"`
}

// IngressConfig is a ingress config.
type IngressConfig []IngressRule

// IngressRule is a ingress rule.
type IngressRule struct {
	// Defines a source subnet.
	Subnet netip.Prefix `yaml:"subnet" docgen:"{'in':'1.7','pattern':'^[0-9a-f.:]+/\d{1,3}$'}"`
	Except Prefix       `yaml:"except,omitempty" docgen:"{'in':'1.7','pattern':'^[0-9a-f.:]+/\d{1,3}$'}"`
}

// Prefix is a wrapper for netip.Prefix.
//
// It implements IsZero() so that yaml.Marshal correctly skips empty values.
type Prefix struct {
	netip.Prefix
}

// IsZero implements yaml.IsZeroer interface.
func (n Prefix) IsZero() bool {
	return n.Prefix == netip.Prefix{}
}

// NewRuleConfigV1Alpha1 creates a new RuleConfig config document.
func NewRuleConfigV1Alpha1() *RuleConfigV1Alpha1 {
	return &RuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RuleConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleRuleConfigV1Alpha1() *RuleConfigV1Alpha1 {
	cfg := NewRuleConfigV1Alpha1()
	cfg.MetaName = "ingress-apid"
	cfg.PortSelector.Protocol = nethelpers.ProtocolTCP
	cfg.PortSelector.Ports = PortRanges{
		{Lo: 50000, Hi: 50000},
	}
	cfg.Ingress = IngressConfig{
		{
			Subnet: netip.MustParsePrefix("192.168.0.0/16"),
		},
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *RuleConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *RuleConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *RuleConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.MetaName == "" {
		return nil, errors.New("name is required")
	}

	if len(s.PortSelector.Ports) == 0 {
		return nil, errors.New("portSelector.ports is required")
	}

	if err := s.PortSelector.Ports.Validate(); err != nil {
		return nil, err
	}

	for _, rule := range s.Ingress {
		if !rule.Subnet.IsValid() {
			return nil, fmt.Errorf("invalid subnet: %s", rule.Subnet)
		}

		if !value.IsZero(rule.Except) && !rule.Except.IsValid() {
			return nil, fmt.Errorf("invalid except: %s", rule.Except)
		}
	}

	return nil, nil
}

// NetworkRuleConfigSignal implements config.NetworkRuleConfigSignal interface.
func (s *RuleConfigV1Alpha1) NetworkRuleConfigSignal() {}

// Rules implements config.NetworkRuleConfigRules interface.
func (s *RuleConfigV1Alpha1) Rules() []config.NetworkRule {
	return []config.NetworkRule{s}
}

// Protocol implements config.NetworkRule interface.
func (s *RuleConfigV1Alpha1) Protocol() nethelpers.Protocol {
	return s.PortSelector.Protocol
}

// PortRanges implements config.NetworkRule interface.
func (s *RuleConfigV1Alpha1) PortRanges() [][2]uint16 {
	return xslices.Map(s.PortSelector.Ports, func(pr PortRange) [2]uint16 {
		return [2]uint16{pr.Lo, pr.Hi}
	})
}

// Subnets implements config.NetworkRule interface.
func (s *RuleConfigV1Alpha1) Subnets() []netip.Prefix {
	return xslices.Map(s.Ingress, func(rule IngressRule) netip.Prefix {
		return rule.Subnet
	})
}

// ExceptSubnets implements config.NetworkRule interface.
func (s *RuleConfigV1Alpha1) ExceptSubnets() []netip.Prefix {
	return xslices.Map(
		xslices.Filter(
			s.Ingress,
			func(rule IngressRule) bool {
				return rule.Except.IsValid()
			},
		),
		func(rule IngressRule) netip.Prefix {
			return rule.Except.Prefix
		},
	)
}
