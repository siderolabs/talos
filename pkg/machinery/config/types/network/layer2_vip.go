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
)

// Layer2VIPKind is a Layer2VIP config document kind.
const Layer2VIPKind = "Layer2VIPConfig"

func init() {
	registry.Register(Layer2VIPKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &Layer2VIPConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkLayer2VIPConfig = &Layer2VIPConfigV1Alpha1{}
	_ config.ConflictingDocument    = &Layer2VIPConfigV1Alpha1{}
	_ config.NamedDocument          = &Layer2VIPConfigV1Alpha1{}
	_ config.Validator              = &Layer2VIPConfigV1Alpha1{}
)

// Layer2VIPConfigV1Alpha1 is a config document to configure virtual IP using Layer 2 (Ethernet) advertisement.
//
//	description: |
//	 Virtual IP configuration should be used only on controlplane nodes to provide virtual IP for Kubernetes API server.
//	 Any other use cases are not supported and may lead to unexpected behavior.
//	 Virtual IP will be announced from only one node at a time using gratuitous ARP announcements for IPv4.
//	examples:
//	  - value: exampleLayer2VIPConfigV1Alpha1()
//	alias: Layer2VIPConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/Layer2VIPConfig
type Layer2VIPConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//    IP address to be advertised as a Layer 2 VIP.
	//
	//   examples:
	//    - value: >
	//       "192.168.100.1"
	//    - value: >
	//       "fd00::1"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Name of the link to assign the VIP to.
	//
	//     Selector must match exactly one link, otherwise an error is returned.
	//     If multiple selectors match the same link, the first one is used.
	LinkName string `yaml:"link"`
}

// NewLayer2VIPConfigV1Alpha1 creates a new Layer2VIPConfig config document.
func NewLayer2VIPConfigV1Alpha1(name string) *Layer2VIPConfigV1Alpha1 {
	return &Layer2VIPConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       Layer2VIPKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleLayer2VIPConfigV1Alpha1() *Layer2VIPConfigV1Alpha1 {
	cfg := NewLayer2VIPConfigV1Alpha1("10.3.0.1")
	cfg.LinkName = "enp0s2"

	return cfg
}

// Clone implements config.Document interface.
func (s *Layer2VIPConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *Layer2VIPConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Validate implements config.Validator interface.
func (s *Layer2VIPConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	} else if _, err := netip.ParseAddr(s.MetaName); err != nil {
		errs = errors.Join(errs, fmt.Errorf("name must be a valid IP address: %w", err))
	}

	if s.LinkName == "" {
		errs = errors.Join(errs, errors.New("link must be specified"))
	}

	return warnings, errs
}

// Link implements config.NetworkLayer2VIPConfig interface.
func (s *Layer2VIPConfigV1Alpha1) Link() string {
	return s.LinkName
}

// VIP implements config.NetworkLayer2VIPConfig interface.
func (s *Layer2VIPConfigV1Alpha1) VIP() netip.Addr {
	addr, _ := netip.ParseAddr(s.MetaName) //nolint:errcheck // already validated

	return addr
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *Layer2VIPConfigV1Alpha1) ConflictsWithKinds() []string {
	return []string{HCloudVIPKind}
}
