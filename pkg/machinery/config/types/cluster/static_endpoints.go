// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// StaticEndpointsKind is a static endpoints config document kind.
const StaticEndpointsKind = "StaticEndpointsConfig"

func init() {
	registry.Register(StaticEndpointsKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &StaticEndpointsConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.StaticEndpointsConfig = &StaticEndpointsConfigV1Alpha1{}
	_ config.Validator             = &StaticEndpointsConfigV1Alpha1{}
)

// StaticEndpointsConfigV1Alpha1 is a config document to provide a static list of control plane endpoints.
//
//	description: |
//	  Talos discovers control plane endpoints dynamically from the discovery service, the Kubernetes API server endpoints, and the cluster control plane endpoint.
//	  The control plane endpoints are used by worker nodes to reach trustd, and by control plane nodes to join etcd.
//	  This document provides a static list of control plane node addresses and hostnames which is merged with the dynamically discovered endpoints.
//	  It is useful when the discovery service is not available, or in clusters without Kubernetes.
//
//	  The list is not trusted: every connection is authenticated with TLS, and unreachable endpoints are skipped.
//	  Endpoints don't include a port, as the port is defined by the service being accessed.
//	examples:
//	  - value: exampleStaticEndpointsConfigV1Alpha1()
//	alias: StaticEndpointsConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/StaticEndpointsConfig
type StaticEndpointsConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     List of control plane node IP addresses.
	//   examples:
	//    - value: >
	//       []netip.Addr{netip.MustParseAddr("10.0.0.11"), netip.MustParseAddr("10.0.0.12"), netip.MustParseAddr("2001:db8::13")}
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+$
	EndpointAddresses []netip.Addr `yaml:"addresses,omitempty"`
	//   description: |
	//     List of control plane node hostnames.
	//     The hostnames are resolved by the services connecting to them.
	//   examples:
	//    - value: >
	//       []string{"cp1.example.com", "cp2.example.com"}
	EndpointHostnames []string `yaml:"hostnames,omitempty"`
}

// NewStaticEndpointsConfigV1Alpha1 creates a new static endpoints config document.
func NewStaticEndpointsConfigV1Alpha1() *StaticEndpointsConfigV1Alpha1 {
	return &StaticEndpointsConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       StaticEndpointsKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleStaticEndpointsConfigV1Alpha1() *StaticEndpointsConfigV1Alpha1 {
	cfg := NewStaticEndpointsConfigV1Alpha1()
	cfg.EndpointAddresses = []netip.Addr{
		netip.MustParseAddr("10.0.0.11"),
		netip.MustParseAddr("10.0.0.12"),
		netip.MustParseAddr("10.0.0.13"),
	}
	cfg.EndpointHostnames = []string{"cp.example.com"}

	return cfg
}

// Clone implements config.Document interface.
func (s *StaticEndpointsConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Addresses implements config.StaticEndpointsConfig interface.
func (s *StaticEndpointsConfigV1Alpha1) Addresses() []netip.Addr {
	if s == nil {
		return nil
	}

	return s.EndpointAddresses
}

// Hostnames implements config.StaticEndpointsConfig interface.
func (s *StaticEndpointsConfigV1Alpha1) Hostnames() []string {
	if s == nil {
		return nil
	}

	return s.EndpointHostnames
}

// Validate implements config.Validator interface.
func (s *StaticEndpointsConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if len(s.EndpointAddresses) == 0 && len(s.EndpointHostnames) == 0 {
		errs = errors.Join(errs, errors.New("at least one address or hostname is required"))
	}

	for _, addr := range s.EndpointAddresses {
		if !addr.IsValid() || addr.IsUnspecified() {
			errs = errors.Join(errs, fmt.Errorf("invalid address %q", addr))
		}
	}

	for _, hostname := range s.EndpointHostnames {
		if err := validateStaticEndpointHostname(hostname); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return nil, errs
}

func validateStaticEndpointHostname(hostname string) error {
	if hostname == "" {
		return errors.New("hostname can't be empty")
	}

	if _, err := netip.ParseAddr(hostname); err == nil {
		return fmt.Errorf("hostname %q is an IP address, it should be listed in addresses", hostname)
	}

	if len(hostname) > 253 {
		return fmt.Errorf("hostname %q is too long: %d", hostname, len(hostname))
	}

	// a colon is never valid in a hostname, catch host:port and URLs
	if strings.ContainsAny(hostname, ":/") {
		return fmt.Errorf("hostname %q should not include a port or a scheme", hostname)
	}

	return nethelpers.ValidateDNSNameChars(hostname)
}
