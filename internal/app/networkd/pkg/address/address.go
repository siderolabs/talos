/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package address

import (
	"context"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// Addressing provides an interface for abstracting the underlying network
// addressing configuration. Currently dhcp(v4) and static methods are
// supported.
type Addressing interface {
	Name() string
	Discover(context.Context) error
	Address() *net.IPNet
	Mask() net.IPMask
	MTU() uint32
	TTL() time.Duration
	Family() int
	Scope() uint8
	Routes() []*Route
	Resolvers() []net.IP
	Hostname() string
	Link() *net.Interface
}

// Route is a representation of a network route
type Route = dhcpv4.Route
