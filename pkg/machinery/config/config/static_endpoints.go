// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "net/netip"

// StaticEndpointsConfig defines a static list of control plane endpoints.
//
// The endpoints complement the dynamically discovered ones (discovery service, Kubernetes API server
// endpoints, control plane endpoint), and are used to reach other control plane nodes, e.g.
// for trustd on worker nodes and etcd join on control plane nodes.
type StaticEndpointsConfig interface {
	// Addresses returns the list of static control plane IP addresses.
	Addresses() []netip.Addr
	// Hostnames returns the list of static control plane hostnames.
	Hostnames() []string
}
