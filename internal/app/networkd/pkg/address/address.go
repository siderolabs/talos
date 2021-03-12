// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package address

import (
	"context"
	"log"
	"net"
	"time"
)

// Addressing provides an interface for abstracting the underlying network
// addressing configuration. Currently dhcp(v4) and static methods are
// supported.
type Addressing interface {
	Address() *net.IPNet
	Discover(context.Context, *log.Logger, *net.Interface) error
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

// Route is a representation of a network route.
type Route struct {
	// Destination is the destination network this route provides.
	Destination *net.IPNet

	// Gateway is the router through which the destination may be reached.
	// This option is exclusive of Interface
	Gateway net.IP

	// Interface indicates the route is an interface route, and traffic destinted for the Gateway should be sent through the given network interface.
	// This option is exclusive of Gateway.
	Interface string

	// Metric indicates the "distance" to the destination through this route.
	// This is an integer which allows the control of priority in the case of multiple routes to the same destination.
	Metric uint32
}
