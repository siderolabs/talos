// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"net/netip"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// KubespanEndpointsKind is a KubeSpan endpoints document kind.
const KubespanEndpointsKind = "KubeSpanEndpointsConfig"

func init() {
	registry.Register(KubespanEndpointsKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubespanEndpointsConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.KubespanConfig = &KubespanEndpointsConfigV1Alpha1{}
)

// KubespanEndpointsConfigV1Alpha1 is a config document to configure KubeSpan endpoints.
//
//	examples:
//	  - value: exampleKubespanEndpointsV1Alpha1()
//	alias: KubeSpanEndpointsConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeSpanEndpoints
type KubespanEndpointsConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     A list of extra Wireguard endpoints to announce from this machine.
	//
	//     Talos automatically adds endpoints based on machine addresses, public IP, etc.
	//     This field allows to add extra endpoints which are managed outside of Talos, e.g. NAT mapping.
	//   schema:
	//     type: array
	//     items:
	//       type: string
	ExtraAnnouncedEndpointsConfig []netip.AddrPort `yaml:"extraAnnouncedEndpoints"`
}

// NewKubespanEndpointsV1Alpha1 creates a new KubespanEndpoints config document.
func NewKubespanEndpointsV1Alpha1() *KubespanEndpointsConfigV1Alpha1 {
	return &KubespanEndpointsConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       KubespanEndpointsKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleKubespanEndpointsV1Alpha1() *KubespanEndpointsConfigV1Alpha1 {
	cfg := NewKubespanEndpointsV1Alpha1()
	cfg.ExtraAnnouncedEndpointsConfig = []netip.AddrPort{
		netip.MustParseAddrPort("192.168.13.46:52000"),
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubespanEndpointsConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// ExtraAnnouncedEndpoints implements KubespanConfig interface.
func (s *KubespanEndpointsConfigV1Alpha1) ExtraAnnouncedEndpoints() []netip.AddrPort {
	return slices.Clone(s.ExtraAnnouncedEndpointsConfig)
}
