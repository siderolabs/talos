// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides resources which describe networking subsystem state.
package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// NamespaceName contains resources related to networking.
const NamespaceName resource.Namespace = "network"

// ConfigNamespaceName contains umerged resources related to networking generate from the configuration.
//
// Resources in the ConfigNamespaceName namespace are merged to produce final versions in the NamespaceName namespace.
const ConfigNamespaceName resource.Namespace = "network-config"

// AddressID builds ID (primary key) for the address.
func AddressID(linkName string, addr netaddr.IPPrefix) string {
	return fmt.Sprintf("%s/%s", linkName, addr)
}

// LinkID builds ID (primary key) for the link (interface).
func LinkID(linkName string) string {
	return linkName
}

// RouteID builds ID (primary key) for the route.
func RouteID(table nethelpers.RoutingTable, family nethelpers.Family, destination netaddr.IPPrefix, gateway netaddr.IP, priority uint32) string {
	dst, _ := destination.MarshalText() //nolint:errcheck
	gw, _ := gateway.MarshalText()      //nolint:errcheck

	tablePrefix := ""

	if table != nethelpers.TableMain {
		tablePrefix = fmt.Sprintf("%s/", table)
	}

	return fmt.Sprintf("%s%s/%s/%s/%d", tablePrefix, family, string(gw), string(dst), priority)
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
	LinkKindWireguard = "wireguard"
)
