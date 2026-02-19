// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network

//docgen:jsonschema

import (
	"errors"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// DHCPv4Kind is a DHCPv4 config document kind.
const DHCPv4Kind = "DHCPv4Config"

func init() {
	registry.Register(DHCPv4Kind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &DHCPv4ConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkDHCPv4Config = &DHCPv4ConfigV1Alpha1{}
	_ config.NamedDocument       = &DHCPv4ConfigV1Alpha1{}
	_ config.Validator           = &DHCPv4ConfigV1Alpha1{}
)

// DHCPv4ConfigV1Alpha1 is a config document to configure DHCPv4 on a network link.
//
//	examples:
//	  - value: exampleDHCPv4ConfigV1Alpha1()
//	alias: DHCPv4Config
//	schemaRoot: true
//	schemaMeta: v1alpha1/DHCPv4Config
type DHCPv4ConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the link (interface).
	//
	//   examples:
	//    - value: >
	//       "enp0s2"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     An optional metric for the routes received from the DHCP server.
	//
	//     Lower values indicate higher priority.
	//     Default value is 1024.
	ConfigRouteMetric uint32 `yaml:"routeMetric,omitempty"`
	//   description: |
	//     Ignore hostname received from the DHCP server.
	ConfigIgnoreHostname *bool `yaml:"ignoreHostname,omitempty"`
	//   description: |
	//     Client identifier to use when communicating with DHCP servers.
	//
	//     Defaults to 'mac' if not set.
	//   values:
	//     - "none"
	//     - "mac"
	//     - "duid"
	ConfigClientIdentifier *nethelpers.ClientIdentifier `yaml:"clientIdentifier,omitempty"`
	//   description: |
	//     Raw value of the DUID to use as client identifier.
	//
	//     This field is only used if 'clientIdentifier' is set to 'duid'.
	//   examples:
	//    - value: >
	//       "00:01:00:01:23:45:67:89:ab:cd:ef:01:23:45"
	//   schema:
	//     type: string
	//     pattern: ^([0-9a-fA-F]{2}(:[0-9a-fA-F]{2})+)$
	ConfigDUIDRaw nethelpers.HardwareAddr `yaml:"duidRaw,omitempty"`
}

// NewDHCPv4ConfigV1Alpha1 creates a new DHCPv4Config config document.
func NewDHCPv4ConfigV1Alpha1(name string) *DHCPv4ConfigV1Alpha1 {
	return &DHCPv4ConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       DHCPv4Kind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleDHCPv4ConfigV1Alpha1() *DHCPv4ConfigV1Alpha1 {
	cfg := NewDHCPv4ConfigV1Alpha1("enp0s2")
	cfg.ConfigClientIdentifier = new(nethelpers.ClientIdentifierMAC)

	return cfg
}

// Clone implements config.Document interface.
func (s *DHCPv4ConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *DHCPv4ConfigV1Alpha1) Name() string {
	return s.MetaName
}

// NetworkDHCPConfig implements config.NetworkDHCPConfig interface.
func (s *DHCPv4ConfigV1Alpha1) NetworkDHCPConfig() {}

// NetworkDHCPv4Config implements config.NetworkDHCPv4Config interface.
func (s *DHCPv4ConfigV1Alpha1) NetworkDHCPv4Config() {}

// RouteMetric returns the route metric.
func (s *DHCPv4ConfigV1Alpha1) RouteMetric() optional.Optional[uint32] {
	if s.ConfigRouteMetric == 0 {
		return optional.None[uint32]()
	}

	return optional.Some(s.ConfigRouteMetric)
}

// IgnoreHostname returns whether to ignore hostname from DHCP server.
func (s *DHCPv4ConfigV1Alpha1) IgnoreHostname() optional.Optional[bool] {
	if s.ConfigIgnoreHostname == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.ConfigIgnoreHostname)
}

// ClientIdentifier returns the client identifier.
func (s *DHCPv4ConfigV1Alpha1) ClientIdentifier() nethelpers.ClientIdentifier {
	if s.ConfigClientIdentifier == nil {
		return nethelpers.ClientIdentifierMAC
	}

	return *s.ConfigClientIdentifier
}

// DUIDRaw returns the DUID raw value.
func (s *DHCPv4ConfigV1Alpha1) DUIDRaw() optional.Optional[nethelpers.HardwareAddr] {
	if len(s.ConfigDUIDRaw) == 0 {
		return optional.None[nethelpers.HardwareAddr]()
	}

	return optional.Some(s.ConfigDUIDRaw)
}

// Validate implements config.Validator interface.
func (s *DHCPv4ConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if len(s.ConfigDUIDRaw) > 0 && pointer.SafeDeref(s.ConfigClientIdentifier) != nethelpers.ClientIdentifierDUID {
		errs = errors.Join(errs, errors.New("duidRaw can only be set if clientIdentifier is 'duid'"))
	}

	if pointer.SafeDeref(s.ConfigClientIdentifier) == nethelpers.ClientIdentifierDUID && len(s.ConfigDUIDRaw) == 0 {
		errs = errors.Join(errs, errors.New("duidRaw must be set if clientIdentifier is 'duid'"))
	}

	return warnings, errs
}
