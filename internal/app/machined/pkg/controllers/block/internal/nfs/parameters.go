// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nfs resolves parameters required by the Linux NFS fs_context API.
package nfs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Resolver resolves NFS server hostnames.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// ResolveMountParameters adds the numeric server address required by the Linux NFS fs_context API.
func ResolveMountParameters(ctx context.Context, source string, parameters []block.ParameterSpec, resolver Resolver) ([]block.ParameterSpec, error) {
	host, err := sourceHost(source)
	if err != nil {
		return nil, err
	}

	resolved, err := resolveAddress(ctx, addressHost(host, parameters), requestedFamily(parameters), resolver)
	if err != nil {
		return nil, err
	}

	result := make([]block.ParameterSpec, 0, len(parameters)+2)

	for _, parameter := range parameters {
		if parameter.Name == "addr" {
			continue
		}

		result = append(result, parameter)
	}

	// mount.nfs derives the netid from the address it resolved when the transport is not configured,
	// so do the same instead of leaving the kernel on its bare default.
	if !hasParameter(parameters, "proto") {
		result = append(result, block.NewStringParameter("proto", netid(resolved)))
	}

	result = append(result, block.NewStringParameter("addr", resolved.String()))

	return result, nil
}

func sourceHost(source string) (string, error) {
	host, export, err := net.SplitHostPort(source)
	if err != nil {
		return "", fmt.Errorf("invalid NFS source %q: %w", source, err)
	}

	if host == "" || export == "" {
		return "", fmt.Errorf("invalid NFS source %q", source)
	}

	return host, nil
}

func addressHost(sourceHost string, parameters []block.ParameterSpec) string {
	for _, parameter := range parameters {
		if parameter.Name != "addr" || parameter.String == nil || *parameter.String == "" {
			continue
		}

		sourceHost = *parameter.String
	}

	return sourceHost
}

func resolveAddress(ctx context.Context, host string, family addressFamily, resolver Resolver) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap()

		if !family.matches(addr) {
			return netip.Addr{}, fmt.Errorf("NFS server %q is not an %s address", host, family)
		}

		return addr, nil
	}

	if resolver == nil {
		return netip.Addr{}, errors.New("NFS hostname resolver is not configured")
	}

	addrs, err := resolver.LookupNetIP(ctx, family.network(), host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to resolve NFS server %q: %w", host, err)
	}

	for _, addr := range addrs {
		addr = addr.Unmap()
		if addr.IsValid() && family.matches(addr) {
			return addr, nil
		}
	}

	return netip.Addr{}, fmt.Errorf("NFS server %q resolved to no %s addresses", host, family)
}

// addressFamily is the address family asserted by the netid suffix, e.g. `tcp` vs `tcp6`.
//
// The kernel maps both to the same transport and only checks the asserted family against `addr`,
// so the netid never selects an address family on its own: it constrains the one we resolve.
type addressFamily int

const (
	familyAny addressFamily = iota
	familyIPv4
	familyIPv6
)

// requestedFamily derives the asserted family from `proto`, falling back to `mountproto`: without a
// `mountaddr` parameter the kernel validates the mount netid against the NFS server address too.
func requestedFamily(parameters []block.ParameterSpec) addressFamily {
	family := familyAny

	for _, name := range []string{"mountproto", "proto"} {
		for _, parameter := range parameters {
			if parameter.Name != name || parameter.String == nil {
				continue
			}

			if strings.HasSuffix(*parameter.String, "6") {
				family = familyIPv6
			} else {
				family = familyIPv4
			}
		}
	}

	return family
}

func (f addressFamily) matches(addr netip.Addr) bool {
	switch f {
	case familyIPv4:
		return addr.Is4()
	case familyIPv6:
		return addr.Is6()
	case familyAny:
		return true
	default:
		return true
	}
}

func (f addressFamily) network() string {
	switch f {
	case familyIPv4:
		return "ip4"
	case familyIPv6:
		return "ip6"
	case familyAny:
		return "ip"
	default:
		return "ip"
	}
}

func (f addressFamily) String() string {
	switch f {
	case familyIPv4:
		return "IPv4"
	case familyIPv6:
		return "IPv6"
	case familyAny:
		return "IP"
	default:
		return "IP"
	}
}

func netid(addr netip.Addr) string {
	if addr.Is6() {
		return "tcp6"
	}

	return "tcp"
}

func hasParameter(parameters []block.ParameterSpec, name string) bool {
	return slices.ContainsFunc(parameters, func(parameter block.ParameterSpec) bool {
		return parameter.Name == name
	})
}
