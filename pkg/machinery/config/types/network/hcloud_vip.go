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

// HCloudVIPKind is a HCloudVIP config document kind.
const HCloudVIPKind = "HCloudVIPConfig"

func init() {
	registry.Register(HCloudVIPKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &HCloudVIPConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkHCloudVIPConfig = &HCloudVIPConfigV1Alpha1{}
	_ config.ConflictingDocument    = &HCloudVIPConfigV1Alpha1{}
	_ config.NamedDocument          = &HCloudVIPConfigV1Alpha1{}
	_ config.Validator              = &HCloudVIPConfigV1Alpha1{}
)

// HCloudVIPConfigV1Alpha1 is a config document to configure virtual IP using Hetzner Cloud APIs for announcement.
//
//	description: |
//	 Virtual IP configuration should be used only on controlplane nodes to provide virtual IP for Kubernetes API server.
//	 Any other use cases are not supported and may lead to unexpected behavior.
//	 Virtual IP will be announced from only one node at a time using Hetzner Cloud APIs.
//	examples:
//	  - value: exampleHCloudVIPConfigV1Alpha1()
//	alias: HCloudVIPConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/HCloudVIPConfig
type HCloudVIPConfigV1Alpha1 struct {
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
	//   description: |
	//     Specifies the Hetzner Cloud API Token.
	APIToken string `yaml:"apiToken"`
}

// NewHCloudVIPConfigV1Alpha1 creates a new HCloudVIPConfig config document.
func NewHCloudVIPConfigV1Alpha1(name string) *HCloudVIPConfigV1Alpha1 {
	return &HCloudVIPConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       HCloudVIPKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleHCloudVIPConfigV1Alpha1() *HCloudVIPConfigV1Alpha1 {
	cfg := NewHCloudVIPConfigV1Alpha1("int0")
	cfg.LinkName = "enp0s2"
	cfg.APIToken = "my-secret-token"

	return cfg
}

// Clone implements config.Document interface.
func (s *HCloudVIPConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *HCloudVIPConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Validate implements config.Validator interface.
func (s *HCloudVIPConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
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

	if s.APIToken == "" {
		errs = errors.Join(errs, errors.New("apiToken must be specified"))
	}

	return warnings, errs
}

// Link implements config.NetworkHCloudVIPConfig interface.
func (s *HCloudVIPConfigV1Alpha1) Link() string {
	return s.LinkName
}

// VIP implements config.NetworkHCloudVIPConfig interface.
func (s *HCloudVIPConfigV1Alpha1) VIP() netip.Addr {
	addr, _ := netip.ParseAddr(s.MetaName) //nolint:errcheck // already validated

	return addr
}

// HCloudAPIToken implements config.NetworkHCloudVIPConfig interface.
func (s *HCloudVIPConfigV1Alpha1) HCloudAPIToken() string {
	return s.APIToken
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *HCloudVIPConfigV1Alpha1) ConflictsWithKinds() []string {
	return []string{Layer2VIPKind}
}
