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
	Address() *net.IPNet
	Discover(context.Context) error
	Family() int
	Hostname() string
	Link() *net.Interface
	MTU() uint32
	Mask() net.IPMask
	Name() string
	Resolvers() []net.IP
	Routes() []*Route
	Scope() uint8
	TTL() time.Duration
	Valid() bool
}

// Route is a representation of a network route
type Route = dhcpv4.Route
