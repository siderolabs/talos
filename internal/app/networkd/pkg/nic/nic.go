// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nic provides a way to describe and configure a network interface.
package nic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink"
	"github.com/jsimonetti/rtnetlink/rtnl"
	"github.com/mdlayher/netlink"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/retry"
)

const (
	// ref: https://tools.ietf.org/html/rfc791

	// MinimumMTU is the lowest allowed MTU for an interface
	MinimumMTU = 68
	// MaximumMTU is the highest allowed MTU for an interface
	MaximumMTU = 65536
)

// NetworkInterface provides an abstract configuration representation for a
// network interface.
type NetworkInterface struct {
	Name          string
	Type          int
	Ignore        bool
	Bonded        bool
	MTU           uint32
	Link          *net.Interface
	SubInterfaces []*net.Interface
	AddressMethod []address.Addressing
	BondSettings  *netlink.AttributeEncoder

	rtConn   *rtnetlink.Conn
	rtnlConn *rtnl.Conn
}

// New returns a NetworkInterface with all of the given setter options applied.
func New(setters ...Option) (*NetworkInterface, error) {
	// Default interface setup
	iface := defaultOptions()

	// Configure interface with any specified options
	var result *multierror.Error
	for _, setter := range setters {
		result = multierror.Append(result, setter(iface))
	}

	// TODO: May need to look at switching this around to filter by Interface.HardwareAddr
	// Ensure we have an interface name defined
	if iface.Name == "" {
		result = multierror.Append(result, errors.New("interface must have a name"))
	}

	// If no addressing methods have been configured, default to DHCP
	if len(iface.AddressMethod) == 0 {
		iface.AddressMethod = append(iface.AddressMethod, &address.DHCP{})
	}

	// Handle netlink connection
	conn, err := rtnl.Dial(nil)
	if err != nil {
		result = multierror.Append(result, err)
		return nil, result.ErrorOrNil()
	}

	iface.rtnlConn = conn

	// Need rtnetlink for MTU and bond settings
	nlConn, err := rtnetlink.Dial(nil)
	if err != nil {
		result = multierror.Append(result, err)
		return nil, result.ErrorOrNil()
	}

	iface.rtConn = nlConn

	return iface, result.ErrorOrNil()
}

// IsIgnored checks the network interface to see if it should be ignored and not configured.
func (n *NetworkInterface) IsIgnored() bool {
	if n.Ignore || kernel.ProcCmdline().Get(constants.KernelParamNetworkInterfaceIgnore).Contains(n.Name) {
		return true
	}

	return false
}

// Create creates the underlying link if it does not already exist.
func (n *NetworkInterface) Create() error {
	iface, err := net.InterfaceByName(n.Name)
	if err == nil {
		n.Link = iface
		return err
	}

	if err = n.createLink(); err != nil {
		return err
	}

	iface, err = net.InterfaceByName(n.Name)
	if err != nil {
		return err
	}

	n.Link = iface

	return nil
}

// Configure is used to set the link state and configure any necessary
// bond settings ( ex, mode ).
func (n *NetworkInterface) Configure() (err error) {
	if n.IsIgnored() {
		return err
	}

	if n.Bonded {
		if err = n.configureBond(n.Link.Index, n.BondSettings); err != nil {
			return err
		}

		bondIndex := proto.Uint32(uint32(n.Link.Index))

		if err = n.enslaveLink(bondIndex, n.SubInterfaces...); err != nil {
			return err
		}
	}

	if err = n.rtnlConn.LinkUp(n.Link); err != nil {
		return err
	}

	// Wait for link to report up
	err = retry.Exponential(30*time.Second, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
		var link *net.Interface

		link, err = n.rtnlConn.LinkByIndex(n.Link.Index)
		if err != nil {
			// nolint: errcheck
			retry.UnexpectedError(err)
		}

		if link.Flags&net.FlagUp != net.FlagUp {
			return retry.ExpectedError(fmt.Errorf("link is not up %s", n.Link.Name))
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to bring up interface%s: %w", n.Link.Name, err)
	}

	return err
}

// Addressing handles the address method for a configured interface ( dhcp/static ).
// This is inclusive of the address itself as well as any defined routes.
func (n *NetworkInterface) Addressing() error {
	if n.IsIgnored() {
		return nil
	}

	for _, method := range n.AddressMethod {
		if err := n.configureInterface(method); err != nil {
			// Treat as non fatal error when failing to configure an interface
			return nil
		}

		if !method.Valid() {
			return nil
		}
	}

	return nil
}

// Renew is the mechanism for keeping a dhcp lease active.
func (n *NetworkInterface) Renew() {
	for _, method := range n.AddressMethod {
		if method.TTL() == 0 {
			continue
		}

		go n.renew(method)
	}
}

// renew sets up the looping to ensure we keep the addressing information
// up to date. We attempt to do our first reconfiguration halfway through
// address TTL. If that fails, we'll continue to attempt to retry every
// halflife.
func (n *NetworkInterface) renew(method address.Addressing) {
	renewDuration := method.TTL() / 2

	var err error

	for {
		<-time.After(renewDuration)

		if err = n.configureInterface(method); err != nil {
			renewDuration = (renewDuration / 2)
		} else {
			renewDuration = method.TTL() / 2
		}
	}
}

// configureInterface handles the actual address discovery mechanism and
// netlink interaction to configure the interface.
// nolint: gocyclo
func (n *NetworkInterface) configureInterface(method address.Addressing) error {
	var err error
	if err = method.Discover(context.Background(), n.Link); err != nil {
		return err
	}

	// Set link MTU if we got a response
	if err = n.setMTU(method.Link().Index, method.MTU()); err != nil {
		return err
	}

	// Check to see if we need to configure the address
	addrs, err := n.rtnlConn.Addrs(method.Link(), method.Family())
	if err != nil {
		return err
	}

	addressExists := false

	for _, addr := range addrs {
		if method.Address().String() == addr.String() {
			addressExists = true
			break
		}
	}

	if !addressExists {
		if err = n.rtnlConn.AddrAdd(method.Link(), method.Address()); err != nil {
			switch err := err.(type) {
			case *netlink.OpError:
				if !os.IsExist(err.Err) && err.Err != syscall.ESRCH {
					return err
				}
			default:
				return fmt.Errorf("failed to add address (already exists) %+v to %s: %v", method.Address(), method.Link().Name, err)
			}
		}
	}

	// Add any routes
	for _, r := range method.Routes() {
		// Any errors here would be non-fatal, so we'll just ignore them
		// nolint: errcheck
		n.rtnlConn.RouteAddSrc(method.Link(), *r.Dest, method.Address(), r.Router)
	}

	return nil
}

// Reset removes addressing configuration from a given link.
func (n *NetworkInterface) Reset() {
	var (
		err  error
		link *net.Interface
		nets []*net.IPNet
	)

	link, err = net.InterfaceByName(n.Name)
	if err != nil {
		return
	}

	if nets, err = n.rtnlConn.Addrs(link, 0); err != nil {
		return
	}

	for _, ipnet := range nets {
		if err = n.rtnlConn.AddrDel(link, ipnet); err != nil {
			continue
		}
	}

	// nolint: errcheck
	n.rtnlConn.LinkDown(link)
}
