// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides resources which describe networking subsystem state.
package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"inet.af/netaddr"
)

// NamespaceName contains resources related to networking.
const NamespaceName resource.Namespace = "network"

// AddressID builds ID (primary key) for the address.
func AddressID(linkName string, addr netaddr.IPPrefix) string {
	return fmt.Sprintf("%s/%s", linkName, addr)
}
