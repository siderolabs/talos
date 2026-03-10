// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// RoutingRuleKind is a RoutingRule config document kind.
const RoutingRuleKind = "RoutingRuleConfig"

func init() {
	registry.Register(RoutingRuleKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &RoutingRuleConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkRoutingRuleConfig = &RoutingRuleConfigV1Alpha1{}
	_ config.NamedDocument            = &RoutingRuleConfigV1Alpha1{}
	_ config.Validator                = &RoutingRuleConfigV1Alpha1{}
)

// RoutingRuleConfigV1Alpha1 is a config document to configure Linux policy routing rules.
//
//	examples:
//	  - value: exampleRoutingRuleConfigV1Alpha1()
//	alias: RoutingRuleConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/RoutingRuleConfig
type RoutingRuleConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Priority of the routing rule.
	//     Lower values are matched first.
	//     Must be between 1 and 32765 (excluding reserved priorities [0 32500 32501 32766 32767]).
	//     Must be unique across all routing rules in the configuration.
	//
	//   examples:
	//    - value: "1000"
	//   schemaRequired: true
	//   schema:
	//     type: string
	RulePriority string `yaml:"name"`
	//   description: |
	//     Source address prefix to match.
	//     If empty, matches all sources.
	//
	//   examples:
	//    - value: >
	//       "10.0.0.0/8"
	//   schema:
	//     type: string
	RuleSrc Prefix `yaml:"src,omitempty"`
	//   description: |
	//     Destination address prefix to match.
	//     If empty, matches all destinations.
	//
	//   examples:
	//    - value: >
	//       "192.168.0.0/16"
	//   schema:
	//     type: string
	RuleDst Prefix `yaml:"dst,omitempty"`
	//   description: |
	//     The routing table to look up if the rule matches.
	//
	//   examples:
	//    - value: "100"
	//   schemaRequired: true
	//   schema:
	//     type: string
	RuleTable nethelpers.RoutingTable `yaml:"table"`
	//   description: |
	//     The action to perform when the rule matches.
	//     Defaults to "unicast" (table lookup).
	//
	//   values:
	//     - unicast
	//     - blackhole
	//     - unreachable
	//     - prohibit
	//   schema:
	//     type: integer
	RuleAction nethelpers.RoutingRuleAction `yaml:"action,omitempty"`
	//   description: |
	//     Match packets arriving on this interface.
	//
	//   examples:
	//    - value: >
	//       "eth0"
	//   schema:
	//     type: string
	RuleIIFName string `yaml:"iifName,omitempty"`
	//   description: |
	//     Match packets going out on this interface.
	//
	//   examples:
	//    - value: >
	//       "eth1"
	//   schema:
	//     type: string
	RuleOIFName string `yaml:"oifName,omitempty"`
	//   description: |
	//     Match packets with this firewall mark value.
	//
	//   examples:
	//    - value: >
	//       uint32(0x100)
	//   schema:
	//     type: integer
	RuleFwMark uint32 `yaml:"fwMark,omitempty"`
	//   description: |
	//     Mask for the firewall mark comparison.
	//
	//   examples:
	//    - value: >
	//       uint32(0xff00)
	//   schema:
	//     type: integer
	RuleFwMask uint32 `yaml:"fwMask,omitempty"`
}

// NewRoutingRuleConfigV1Alpha1 creates a new RoutingRuleConfig config document.
func NewRoutingRuleConfigV1Alpha1(priority uint32) *RoutingRuleConfigV1Alpha1 {
	return &RoutingRuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RoutingRuleKind,
			MetaAPIVersion: "v1alpha1",
		},
		RulePriority: strconv.FormatUint(uint64(priority), 10),
	}
}

func exampleRoutingRuleConfigV1Alpha1() *RoutingRuleConfigV1Alpha1 {
	cfg := NewRoutingRuleConfigV1Alpha1(1000)
	cfg.RuleSrc = Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cfg.RuleTable = nethelpers.RoutingTable(100)
	cfg.RuleAction = nethelpers.RoutingRuleActionUnicast

	return cfg
}

// Clone implements config.Document interface.
func (s *RoutingRuleConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *RoutingRuleConfigV1Alpha1) Name() string {
	return s.RulePriority
}

// RoutingRuleConfig implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) RoutingRuleConfig() {}

var reservedPriorities = []uint32{
	constants.LinuxReservedRulePriorityLocal,
	constants.KubeSpanDefaultRulePriority,
	constants.LinuxReservedRulePriorityMain,
	constants.LinuxReservedRulePriorityDefault,
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *RoutingRuleConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.RulePriority == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	priority, err := strconv.ParseUint(s.RulePriority, 10, 32)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("invalid name: must be priority parsable unsigned integer: %w", err))
	}

	if slices.Contains(reservedPriorities, uint32(priority)) {
		errs = errors.Join(errs, fmt.Errorf("priority must be between 1 and 32765 (excluding reserved priorities %v)", reservedPriorities))
	}

	if s.RuleTable == nethelpers.TableUnspec &&
		(s.RuleAction == nethelpers.RoutingRuleActionUnspec || s.RuleAction == nethelpers.RoutingRuleActionUnicast) {
		errs = errors.Join(errs, errors.New("either table or a non-unicast action must be specified"))
	}

	if s.RuleFwMask != 0 && s.RuleFwMark == 0 {
		errs = errors.Join(errs, errors.New("fwMask requires fwMark to be set"))
	}

	return nil, errs
}

// Src implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) Src() optional.Optional[netip.Prefix] {
	if s.RuleSrc == (Prefix{}) {
		return optional.None[netip.Prefix]()
	}

	return optional.Some(s.RuleSrc.Prefix)
}

// Dst implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) Dst() optional.Optional[netip.Prefix] {
	if s.RuleDst == (Prefix{}) {
		return optional.None[netip.Prefix]()
	}

	return optional.Some(s.RuleDst.Prefix)
}

// Table implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) Table() nethelpers.RoutingTable {
	return s.RuleTable
}

// Priority implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) Priority() uint32 {
	priority, _ := strconv.ParseUint(s.RulePriority, 10, 32) //nolint:errcheck

	return uint32(priority)
}

// Action implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) Action() nethelpers.RoutingRuleAction {
	return s.RuleAction
}

// IIFName implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) IIFName() string {
	return s.RuleIIFName
}

// OIFName implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) OIFName() string {
	return s.RuleOIFName
}

// FwMark implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) FwMark() uint32 {
	return s.RuleFwMark
}

// FwMask implements NetworkRoutingRuleConfig interface.
func (s *RoutingRuleConfigV1Alpha1) FwMask() uint32 {
	return s.RuleFwMask
}
