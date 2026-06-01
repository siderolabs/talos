// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

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
//	  - value: exampleLinkAliasMultipleConfigV1Alpha1()
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
	//    The name can contain a single integer format verb (`%d`) to create multiple aliases
	//    from a single config document. When a format verb is detected, each matched link receives a sequential
	//    alias (e.g. `net0`, `net1`, ...) based on hardware address order of the links.
	//    Links already aliased by a previous config are automatically skipped.
	//
	//   examples:
	//    - value: >
	//       "net0"
	//    - value: >
	//       "private"
	//    - value: >
	//       "net%d"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Selector to match the link to alias.
	//
	//     When the alias name is a fixed string, the selector must match exactly one link.
	//     When the alias name contains a format verb (e.g. `net%d`), the selector may match multiple links
	//     and each match receives a sequential alias.
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

func exampleLinkAliasMultipleConfigV1Alpha1() *LinkAliasConfigV1Alpha1 {
	cfg := NewLinkAliasConfigV1Alpha1("net%d")
	cfg.Selector.Match = exampleLinkSelector3()

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
	} else if s.IsPatternAlias() {
		prefix, suffix, _ := strings.Cut(s.MetaName, "%")
		if suffix != "d" || prefix == "" {
			errs = errors.Join(errs, fmt.Errorf("name %q contains an invalid format verb, use %%d suffix", s.MetaName))
		}
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

// IsPatternAlias returns true if the alias name contains a format verb (e.g. %d)
// indicating this config should create multiple aliases.
func (s *LinkAliasConfigV1Alpha1) IsPatternAlias() bool {
	return strings.ContainsRune(s.MetaName, '%')
}
