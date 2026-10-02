// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides resources which describe networking subsystem state.
package network

import (
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:generate go tool github.com/dmarkham/enumer -type=ConfigLayer,Operator -linecomment -text

//go:generate go tool github.com/siderolabs/talos/tools/redactgen -header-file ../../../../hack/boilerplate.txt -o redact.generated.go .

// NamespaceName contains resources related to networking.
const NamespaceName resource.Namespace = "network"

// ConfigNamespaceName contains umerged resources related to networking generate from the configuration.
//
// Resources in the ConfigNamespaceName namespace are merged to produce final versions in the NamespaceName namespace.
const ConfigNamespaceName resource.Namespace = "network-config"

// DefaultRouteMetric is the default route metric if no metric was specified explicitly.
const DefaultRouteMetric = 1024

// AddressID builds ID (primary key) for the address.
func AddressID(linkName string, addr netip.Prefix) string {
	return fmt.Sprintf("%s/%s", linkName, addr)
}

// LinkID builds ID (primary key) for the link (interface).
func LinkID(linkName string) string {
	return linkName
}

// RouteID builds ID (primary key) for the route spec.
//
// The ID matches the key the kernel keeps a single route for: table, family, destination and priority (metric).
// Gateway, out link and multipath next-hops are attributes of the route, so changing them updates the route
// in place instead of replacing it with a new one.
//
// The destination is masked, as the kernel keys the route by the network prefix: 10.0.0.1/8 and 10.0.0.0/8 are the same route.
func RouteID(table nethelpers.RoutingTable, family nethelpers.Family, destination netip.Prefix, priority uint32) string {
	// Masked returns the zero prefix for an invalid (zero) one
	destination = destination.Masked()

	if destination.Bits() <= 0 {
		// the default route might be specified either as a zero prefix or as an explicit 0.0.0.0/0 (::/0)
		destination = netip.Prefix{}
	}

	if family == nethelpers.FamilyInet6 && priority == 0 {
		// Linux assigns the default metric to IPv6 routes added without an explicit metric
		priority = DefaultRouteMetric
	}

	dst, _ := destination.MarshalText() //nolint:errcheck

	prefix := ""

	if table != nethelpers.TableMain {
		prefix = fmt.Sprintf("%s/", table)
	}

	return fmt.Sprintf("%s%s/%s/%d", prefix, family, string(dst), priority)
}

// RouteStatusID builds ID (primary key) for the route status.
//
// Unlike RouteID, it includes the gateway and the out link (for IPv6), as the kernel might have several
// routes with the same table, destination and priority: e.g. routes created by the kernel itself for each link,
// or routes appended to an existing one.
func RouteStatusID(table nethelpers.RoutingTable, family nethelpers.Family, destination netip.Prefix, gateway netip.Addr, priority uint32, outLinkName string) string {
	dst, _ := destination.MarshalText() //nolint:errcheck
	gw, _ := gateway.MarshalText()      //nolint:errcheck

	prefix := ""

	if table != nethelpers.TableMain {
		prefix = fmt.Sprintf("%s/", table)
	}

	if family == nethelpers.FamilyInet6 {
		prefix += fmt.Sprintf("%s/", outLinkName)
	}

	return fmt.Sprintf("%s%s/%s/%s/%d", prefix, family, string(gw), string(dst), priority)
}

// RoutingRuleID builds ID (primary key) for the routing rule.
func RoutingRuleID(family nethelpers.Family, priority uint32) string {
	return fmt.Sprintf("%s/%05d", family, priority)
}

// OperatorID builds ID (primary key) for the operators.
func OperatorID(spec OperatorSpecSpec) string {
	switch spec.Operator {
	case OperatorVIP:
		return fmt.Sprintf("%s/%s/%s", spec.Operator, spec.LinkName, spec.VIP.IP.String())
	case OperatorDHCP4:
		fallthrough
	case OperatorDHCP6:
		fallthrough
	case OperatorLLDP:
		fallthrough
	default:
		return fmt.Sprintf("%s/%s", spec.Operator, spec.LinkName)
	}
}

// LayeredID builds configuration for the entity at some layer.
func LayeredID(layer ConfigLayer, id string) string {
	return fmt.Sprintf("%s/%s", layer, id)
}

// Link kinds.
const (
	LinkKindVLAN      = "vlan"
	LinkKindBond      = "bond"
	LinkKindBridge    = "bridge"
	LinkKindVRF       = "vrf"
	LinkKindVeth      = "veth"
	LinkKindMacVLAN   = "macvlan"
	LinkKindWireguard = "wireguard"
	LinkKindVXLAN     = "vxlan"
)
