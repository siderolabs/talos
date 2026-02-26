// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// BlackholeRouteKind is a BlackholeRoute config document kind.
const BlackholeRouteKind = "BlackholeRouteConfig"

func init() {
	registry.Register(BlackholeRouteKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &BlackholeRouteConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkBlackholeRouteConfig = &BlackholeRouteConfigV1Alpha1{}
	_ config.NamedDocument               = &BlackholeRouteConfigV1Alpha1{}
	_ config.Validator                   = &BlackholeRouteConfigV1Alpha1{}
)

// BlackholeRouteConfigV1Alpha1 is a config document to configure blackhole routes.
//
//	examples:
//	  - value: exampleBlackholeRouteConfigV1Alpha1()
//	alias: BlackholeRouteConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/BlackholeRouteConfig
type BlackholeRouteConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Route destination as an address prefix.
	//
	//   examples:
	//    - value: >
	//       "10.0.0.0/12"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     The optional metric for the route.
	RouteMetric uint32 `yaml:"metric,omitempty"`
}

// NewBlackholeRouteConfigV1Alpha1 creates a new BlackholeRouteConfig config document.
func NewBlackholeRouteConfigV1Alpha1(name string) *BlackholeRouteConfigV1Alpha1 {
	return &BlackholeRouteConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       BlackholeRouteKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleBlackholeRouteConfigV1Alpha1() *BlackholeRouteConfigV1Alpha1 {
	cfg := NewBlackholeRouteConfigV1Alpha1("10.0.0.0/12")
	cfg.RouteMetric = 100

	return cfg
}

// Clone implements config.Document interface.
func (s *BlackholeRouteConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *BlackholeRouteConfigV1Alpha1) Name() string {
	return s.MetaName
}

// BlackholeRouteConfig implements BlackholeRouteConfig interface.
func (s *BlackholeRouteConfigV1Alpha1) BlackholeRouteConfig() {}

// Validate implements config.Validator interface.
func (s *BlackholeRouteConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if _, err := netip.ParsePrefix(s.MetaName); err != nil {
		errs = errors.Join(errs, fmt.Errorf("name must be a valid address prefix: %w", err))
	}

	return nil, errs
}

// Metric implements NetworkRouteConfig interface.
func (s *BlackholeRouteConfigV1Alpha1) Metric() optional.Optional[uint32] {
	if s.RouteMetric == 0 {
		return optional.None[uint32]()
	}

	return optional.Some(s.RouteMetric)
}
