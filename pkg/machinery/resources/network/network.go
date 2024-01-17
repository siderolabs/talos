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

// RouteID builds ID (primary key) for the route.
func RouteID(table nethelpers.RoutingTable, family nethelpers.Family, destination netip.Prefix, gateway netip.Addr, priority uint32, outLinkName string) string {
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

// OperatorID builds ID (primary key) for the operators.
func OperatorID(operator Operator, linkName string) string {
	return fmt.Sprintf("%s/%s", operator, linkName)
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
	LinkKindWireguard = "wireguard"
)
