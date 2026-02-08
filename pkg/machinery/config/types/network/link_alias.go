// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// LinkAliasKind is a LinkAlias config document kind.
const LinkAliasKind = "LinkAliasConfig"

func init() {
	registry.Register(LinkAliasKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LinkAliasConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkLinkAliasConfig = &LinkAliasConfigV1Alpha1{}
	_ config.NamedDocument          = &LinkAliasConfigV1Alpha1{}
	_ config.Validator              = &LinkAliasConfigV1Alpha1{}
)

// LinkAliasConfigV1Alpha1 is a config document to alias (give a different name) to a physical link.
//
//	examples:
//	  - value: exampleLinkAliasConfigV1Alpha1()
//	alias: LinkAliasConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/LinkAliasConfig
type LinkAliasConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//    Alias for the link.
	//
	//    Don't use system interface names like "eth0", "ens3", "enp0s2", etc. as those may conflict
	//    with existing physical interfaces.
	//
	//   examples:
	//    - value: >
	//       "net0"
	//    - value: >
	//       "private"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Selector to match the link to alias.
	//
	//     By default, the selector must match exactly one link, otherwise the alias is not applied.
	//     Set `requireUniqueMatch` to `false` to allow multiple matches and use the first matching link.
	//     If multiple selectors match the same link, the first one is used.
	Selector LinkSelector `yaml:"selector,omitempty"`
}

// LinkSelector selects a link to alias.
type LinkSelector struct {
	//   description: |
	//     The Common Expression Language (CEL) expression to match the link.
	//   schema:
	//     type: string
	//   examples:
	//    - value: >
	//        exampleLinkSelector1()
	//      name: match links with a specific MAC address
	//    - value: >
	//        exampleLinkSelector2()
	//      name: match links by MAC address prefix
	//    - value: >
	//        exampleLinkSelector3()
	//      name: match links by driver name
	Match cel.Expression `yaml:"match,omitempty"`
	//   description: |
	//     Require the selector to match exactly one link.
	//
	//     When set to `false`, if multiple links match the selector, the first matching link is used.
	//     When set to `true` (default), if multiple links match, the alias is not applied.
	//   schema:
	//     type: boolean
	RequireUniqueMatch *bool `yaml:"requireUniqueMatch,omitempty"`
	//   description: |
	//     Skip links that already have an alias assigned by a previous LinkAliasConfig.
	//
	//     This allows creating sequential aliases like `net0` and `net1` from any N links
	//     by using the same broad selector and relying on processing order.
	//   schema:
	//     type: boolean
	SkipAliasedLinks *bool `yaml:"skipAliasedLinks,omitempty"`
}

// NewLinkAliasConfigV1Alpha1 creates a new LinkAliasConfig config document.
func NewLinkAliasConfigV1Alpha1(name string) *LinkAliasConfigV1Alpha1 {
	return &LinkAliasConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LinkAliasKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleLinkAliasConfigV1Alpha1() *LinkAliasConfigV1Alpha1 {
	cfg := NewLinkAliasConfigV1Alpha1("int0")
	cfg.Selector.Match = exampleLinkSelector2()

	return cfg
}

func exampleLinkSelector1() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"`, celenv.LinkLocator()))
}

func exampleLinkSelector2() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`glob("00:1a:2b:*", mac(link.permanent_addr))`, celenv.LinkLocator()))
}

func exampleLinkSelector3() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`link.driver == "e1000"`, celenv.LinkLocator()))
}

// Clone implements config.Document interface.
func (s *LinkAliasConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *LinkAliasConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Validate implements config.Validator interface.
func (s *LinkAliasConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if !s.Selector.Match.IsZero() {
		if err := s.Selector.Match.ParseBool(celenv.LinkLocator()); err != nil {
			errs = errors.Join(errs, fmt.Errorf("link selector is invalid: %w", err))
		}
	} else {
		errs = errors.Join(errs, errors.New("link selector is required"))
	}

	return warnings, errs
}

// LinkSelector implements config.NetworkLinkAliasConfig interface.
func (s *LinkAliasConfigV1Alpha1) LinkSelector() cel.Expression {
	return s.Selector.Match
}

// RequireUniqueMatch implements config.NetworkLinkAliasConfig interface.
func (s *LinkAliasConfigV1Alpha1) RequireUniqueMatch() bool {
	if s.Selector.RequireUniqueMatch == nil {
		return true
	}

	return *s.Selector.RequireUniqueMatch
}

// SkipAliasedLinks implements config.NetworkLinkAliasConfig interface.
func (s *LinkAliasConfigV1Alpha1) SkipAliasedLinks() bool {
	if s.Selector.SkipAliasedLinks == nil {
		return false
	}

	return *s.Selector.SkipAliasedLinks
}
