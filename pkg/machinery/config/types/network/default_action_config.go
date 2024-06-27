// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// DefaultActionConfig is a default action config document kind.
const DefaultActionConfig = "NetworkDefaultActionConfig"

func init() {
	registry.Register(DefaultActionConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &DefaultActionConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkRuleConfigDefaultAction = &DefaultActionConfigV1Alpha1{}
	_ config.NetworkRuleConfigSignal        = &DefaultActionConfigV1Alpha1{}
)

// DefaultActionConfigV1Alpha1 is a ingress firewall default action configuration document.
//
// docgen:configuration
type DefaultActionConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	// The default action for all configured ingress traffic not explicitly defined.
	Ingress nethelpers.DefaultAction `yaml:"ingress" docgen:"{'in': '1.7', 'values': ['accept', 'block']}"`
}

// NewDefaultActionConfigV1Alpha1 creates a new DefaultActionConfig config document.
func NewDefaultActionConfigV1Alpha1() *DefaultActionConfigV1Alpha1 {
	return &DefaultActionConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       DefaultActionConfig,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleDefaultActionConfigV1Alpha1() *DefaultActionConfigV1Alpha1 {
	cfg := NewDefaultActionConfigV1Alpha1()
	cfg.Ingress = nethelpers.DefaultActionAccept

	return cfg
}

// Clone implements config.Document interface.
func (s *DefaultActionConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// NetworkRuleConfigSignal implements config.NetworkRuleConfigSignal interface.
func (s *DefaultActionConfigV1Alpha1) NetworkRuleConfigSignal() {}

// DefaultAction implements config.NetworkRuleConfigDefaultAction interface.
func (s *DefaultActionConfigV1Alpha1) DefaultAction() nethelpers.DefaultAction {
	return s.Ingress
}
