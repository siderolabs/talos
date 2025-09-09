// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// HostnameKind is a Hostname config document kind.
const HostnameKind = "HostnameConfig"

func init() {
	registry.Register(HostnameKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &HostnameConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkHostnameConfig        = &HostnameConfigV1Alpha1{}
	_ config.Validator                    = &HostnameConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &HostnameConfigV1Alpha1{}
)

// HostnameConfigV1Alpha1 is a config document to configure the hostname: either a static hostname or an automatically generated hostname.
//
//	examples:
//	  - value: exampleHostnameConfigV1Alpha1()
//	  - value: exampleHostnameConfigV1Alpha2()
//	alias: HostnameConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/HostnameConfig
type HostnameConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     A method to automatically generate a hostname for the machine.
	//
	//     There are two methods available:
	//       - `stable` - generates a stable hostname based on machine identity
	//       - `off` - disables automatic hostname generation, Talos will wait for an external source to provide a hostname (DHCP, cloud-init, etc).
	//
	//     Automatic hostnames have the lowest priority over any other hostname sources: DHCP, cloud-init, etc.
	//     Conflicts with `hostname` field.
	//   values:
	//     - "stable"
	//     - "off"
	ConfigAuto *nethelpers.AutoHostnameKind `yaml:"auto,omitempty"`
	//   description: |
	//     A static hostname to set for the machine.
	//
	//     This hostname has the highest priority over any other hostname sources: DHCP, cloud-init, etc.
	//     Conflicts with `auto` field.
	//   examples:
	//    - value: >
	//       "controlplane1"
	//    - value: >
	//       "controlplane1.example.org"
	ConfigHostname string `yaml:"hostname,omitempty"`
}

// NewHostnameConfigV1Alpha1 creates a new HostnameConfig config document.
func NewHostnameConfigV1Alpha1() *HostnameConfigV1Alpha1 {
	return &HostnameConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       HostnameKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleHostnameConfigV1Alpha1() *HostnameConfigV1Alpha1 {
	cfg := NewHostnameConfigV1Alpha1()
	cfg.ConfigHostname = "worker-33"

	return cfg
}

func exampleHostnameConfigV1Alpha2() *HostnameConfigV1Alpha1 {
	cfg := NewHostnameConfigV1Alpha1()
	cfg.ConfigAuto = pointer.To(nethelpers.AutoHostnameKindStable)

	return cfg
}

// Clone implements config.Document interface.
func (s *HostnameConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *HostnameConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if pointer.SafeDeref(s.ConfigAuto) == nethelpers.AutoHostnameKindOff && s.ConfigHostname == "" {
		errs = errors.Join(errs, errors.New("either 'auto' or 'hostname' must be set"))
	}

	if pointer.SafeDeref(s.ConfigAuto) != nethelpers.AutoHostnameKindOff && s.ConfigHostname != "" {
		errs = errors.Join(errs, errors.New("'auto' and 'hostname' cannot be set at the same time"))
	}

	switch pointer.SafeDeref(s.ConfigAuto) {
	case nethelpers.AutoHostnameKindOff, nethelpers.AutoHostnameKindStable:
		// valid values
	case nethelpers.AutoHostnameKindAddr:
		fallthrough
	default:
		errs = errors.Join(errs, fmt.Errorf("invalid value for 'auto': %s", s.ConfigAuto))
	}

	if s.ConfigHostname != "" {
		if len(s.ConfigHostname) > 253 {
			errs = errors.Join(errs, fmt.Errorf("fqdn is too long: %d", len(s.ConfigHostname)))
		}

		hostname, _, _ := strings.Cut(s.ConfigHostname, ".")

		if len(hostname) == 0 || len(hostname) > 63 {
			errs = errors.Join(errs, fmt.Errorf("invalid hostname %q", hostname))
		}
	}

	return nil, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *HostnameConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.Hostname() != "" {
		return errors.New("static hostname is already set in v1alpha1 config")
	}

	if v1alpha1Cfg.AutoHostname() != nethelpers.AutoHostnameKindAddr {
		return errors.New("stable hostname is already set in v1alpha1 config")
	}

	return nil
}

// Hostname implements config.NetworkHostnameConfig interface.
func (s *HostnameConfigV1Alpha1) Hostname() string {
	return s.ConfigHostname
}

// AutoHostname implements config.NetworkHostnameConfig interface.
func (s *HostnameConfigV1Alpha1) AutoHostname() nethelpers.AutoHostnameKind {
	return pointer.SafeDeref(s.ConfigAuto)
}
