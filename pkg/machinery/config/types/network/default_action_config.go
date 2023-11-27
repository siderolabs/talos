// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

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

// DefaultActionConfigV1Alpha1 is a event sink config document.
type DefaultActionConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	Ingress nethelpers.DefaultAction `yaml:"ingress"`
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
