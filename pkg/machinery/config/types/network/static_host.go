// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"net/netip"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// StaticHostKind is a StaticHost config document kind.
const StaticHostKind = "StaticHostConfig"

func init() {
	registry.Register(StaticHostKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &StaticHostConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NamedDocument           = &StaticHostConfigV1Alpha1{}
	_ config.Validator               = &StaticHostConfigV1Alpha1{}
	_ config.NetworkStaticHostConfig = &StaticHostConfigV1Alpha1{}
)

// StaticHostConfigV1Alpha1 is a config document to set /etc/hosts entries.
//
//	examples:
//	  - value: exampleStaticHostConfigV1Alpha1()
//	alias: StaticHostConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/StaticHostConfig
type StaticHostConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     IP address (IPv4 or IPv6) to map the hostnames to.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     List of hostnames to map to the IP address.
	Hostnames []string `yaml:"hostnames"`
}

// NewStaticHostConfigV1Alpha1 creates a new StaticHostConfig config document.
func NewStaticHostConfigV1Alpha1(name string) *StaticHostConfigV1Alpha1 {
	return &StaticHostConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       StaticHostKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleStaticHostConfigV1Alpha1() *StaticHostConfigV1Alpha1 {
	cfg := NewStaticHostConfigV1Alpha1("10.5.0.2")
	cfg.Hostnames = []string{"my-server", "my-server.example.org"}

	return cfg
}

// Clone implements config.Document interface.
func (s *StaticHostConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *StaticHostConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Validate implements config.Validator interface.
func (s *StaticHostConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name is required"))
	}

	if _, err := netip.ParseAddr(s.MetaName); err != nil {
		errs = errors.Join(errs, errors.New("name must be a valid IP address"))
	}

	if len(s.Hostnames) == 0 {
		errs = errors.Join(errs, errors.New("at least one hostname is required"))
	}

	return nil, errs
}

// IP implements ExtraHost interface.
func (s *StaticHostConfigV1Alpha1) IP() string {
	return s.MetaName
}

// Aliases implements ExtraHost interface.
func (s *StaticHostConfigV1Alpha1) Aliases() []string {
	return s.Hostnames
}
