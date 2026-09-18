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

// LinkIngressKind is a NetworkLinkIngress config document kind.
const LinkIngressKind = "NetworkLinkIngressConfig"

func init() {
	registry.Register(LinkIngressKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LinkIngressConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkLinkIngressConfig = &LinkIngressConfigV1Alpha1{}
	_ config.NamedDocument            = &LinkIngressConfigV1Alpha1{}
	_ config.Validator                = &LinkIngressConfigV1Alpha1{}
)

// LinkIngressConfigV1Alpha1 is a config document to filter out the packets coming into the system based on destination IP and interface.
//
//	description: |
//	  Filters incoming packets on a link by destination address: only packets destined to one of
//	  the node's own addresses are accepted, anything else is dropped.
//	  This is meant for clusters using an encapsulating CNI, where pod/service CIDR destinations should never
//	  arrive unencapsulated on an external interface; it is incompatible with native pod IP routing (e.g. BGP).
//
//	  The set of accepted destinations can be overridden with the `destinationAddresses` option.
//	examples:
//	  - value: exampleLinkIngressConfigV1Alpha1()
//	alias: NetworkLinkIngressConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/NetworkLinkIngressConfig
type LinkIngressConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the link (interface) to filter the incoming packets on.
	//   examples:
	//    - value: >
	//       "enp0s2"
	//    - value: >
	//       "enp0s2.35"
	//   schemaRequired: true
	MetaName string `yaml:"name"`

	//   description: |
	//     Destination addresses to accept on this link, as a list of CIDRs.
	//
	//     This is an override: when specified, only packets destined to one of these addresses are
	//     accepted, and the node's own addresses are not implicitly allowed.
	//
	//     An empty list allows no destination at all, i.e. all packets arriving on the link are
	//     dropped.
	//
	//     Default value: the node's own addresses.
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+/\d{1,3}$
	//   examples:
	//    - value: '[]meta.Prefix{{netip.MustParsePrefix("1.2.3.4/32")}}'
	//    - value: '[]meta.Prefix{{netip.MustParsePrefix("192.168.10.0/24")}}'
	DestinationAddressesConfig []meta.Prefix `yaml:"destinationAddresses,omitempty" merge:"replace" talos:"omitonlyifnil"`
}

// NewLinkIngressConfigV1Alpha1 creates a new NetworkLinkIngressConfig config document.
func NewLinkIngressConfigV1Alpha1(name string) *LinkIngressConfigV1Alpha1 {
	return &LinkIngressConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LinkIngressKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleLinkIngressConfigV1Alpha1() *LinkIngressConfigV1Alpha1 {
	cfg := NewLinkIngressConfigV1Alpha1("enp0s2.35")
	cfg.DestinationAddressesConfig = []meta.Prefix{{Prefix: netip.MustParsePrefix("1.2.3.4/32")}}

	return cfg
}

// Clone implements config.Document interface.
func (s *LinkIngressConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *LinkIngressConfigV1Alpha1) Name() string {
	return s.MetaName
}

// DestinationAddresses implements config.NetworkLinkIngressConfig interface.
//
// A nil result means that the option is not set, while an empty non-nil result means that it is
// set to an empty list, i.e. that no destination is allowed on the link.
func (s *LinkIngressConfigV1Alpha1) DestinationAddresses() []netip.Prefix {
	if s.DestinationAddressesConfig == nil {
		return nil
	}

	// not using xslices.Map here, as it returns nil for an empty input
	result := make([]netip.Prefix, 0, len(s.DestinationAddressesConfig))

	for _, configuredPrefix := range s.DestinationAddressesConfig {
		result = append(result, configuredPrefix.Prefix)
	}

	return result
}

// Validate implements config.Validator interface.
func (s *LinkIngressConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("link name must be specified"))
	}

	if s.DestinationAddressesConfig != nil && len(s.DestinationAddressesConfig) == 0 {
		warnings = append(warnings, fmt.Sprintf("network link ingress %q: empty destinationAddresses drops all traffic on the link", s.MetaName))
	}

	for i, configuredPrefix := range s.DestinationAddressesConfig {
		prefix := configuredPrefix.Prefix

		if !prefix.IsValid() {
			errs = errors.Join(errs, fmt.Errorf("destinationAddresses[%d]: invalid prefix", i))

			continue
		}

		if prefix != prefix.Masked() {
			errs = errors.Join(errs, fmt.Errorf("destinationAddresses[%d]: prefix %s must be masked", i, prefix))
		}
	}

	return warnings, errs
}
