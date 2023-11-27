// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
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
type RuleConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	MetaName  string `yaml:"name"`

	PortSelector RulePortSelector `yaml:"portSelector"`
	Ingress      IngressConfig    `yaml:"ingress"`
}

// RulePortSelector is a port selector for the network rule.
type RulePortSelector struct {
	Ports    PortRanges          `yaml:"ports"`
	Protocol nethelpers.Protocol `yaml:"protocol"`
}

// IngressConfig is a ingress config.
type IngressConfig []IngressRule

// IngressRule is a ingress rule.
type IngressRule struct {
	Subnet netip.Prefix `yaml:"subnet"`
	Except netip.Prefix `yaml:"except,omitempty"`
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
		return nil, fmt.Errorf("name is required")
	}

	if len(s.PortSelector.Ports) == 0 {
		return nil, fmt.Errorf("portSelector.ports is required")
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
			return rule.Except
		},
	)
}
