/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

// Probably should put together some map here
// to map number with string()
const (
	Bond = iota
	Single

	// https://tools.ietf.org/html/rfc791
	MinimumMTU = 68
	MaximumMTU = 65535
)

type NetworkInterface struct {
	Name          string
	Type          int
	MTU           int
	Routes        []Route
	SubInterfaces []string
	Addressing    Address
}

type NetworkInfo struct {
	IP  net.IP
	Net *net.IPNet
}

type Address interface {
	Configure(*rtnetlink.Conn, int) error
	TTL() time.Duration
}

type Option func(*NetworkInterface) error

func CreateInterface(setters ...Option) (*NetworkInterface, error) {
	iface := defaultOptions()

	var result *multierror.Error
	for _, setter := range setters {
		result = multierror.Append(setter(iface))
	}

	if iface.Name == "" {
		result = multierror.Append(errors.New("interface must have a name"))
	}

	return iface, result.ErrorOrNil()
}

func defaultOptions() *NetworkInterface {
	return &NetworkInterface{
		Type:       Single,
		MTU:        1500,
		Addressing: &DHCP{},
	}
}

func WithName(o string) Option {
	return func(n *NetworkInterface) (err error) {
		n.Name = o
		return err
	}
}

func WithType(o int) Option {
	return func(n *NetworkInterface) (err error) {
		switch o {
		case Bond:
			n.Type = Bond
		case Single:
			n.Type = Single
		default:
			return errors.New("unsupported network interface type")
		}
		return err
	}
}

func WithMTU(mtu int) Option {
	return func(n *NetworkInterface) (err error) {
		if (mtu < MinimumMTU) || (mtu > MaximumMTU) {
			return errors.New("mtu is out of acceptable range")
		}

		n.MTU = mtu
		return err
	}
}

func WithRoute(route Route) Option {
	return func(n *NetworkInterface) (err error) {
		n.Routes = append(n.Routes, route)
		return err
	}
}

func WithSubInterface(o string) Option {
	return func(n *NetworkInterface) (err error) {
		n.SubInterfaces = append(n.SubInterfaces, o)
		return err
	}
}

func WithAddressing(a Address) Option {
	return func(n *NetworkInterface) (err error) {
		n.Addressing = a
		return err
	}
}

func (n *NetworkInterface) Setup(conn *rtnetlink.Conn) (err error) {
	// Do the necessary to bring up the link
	iface, err := net.InterfaceByName(n.Name)
	if err != nil {
		log.Println("failed net.interfacebyname")
		return err
	}

	msg, err := conn.Link.Get(uint32(iface.Index))
	if err != nil {
		log.Println("failed netlinkconn")
		return err
	}

	// TODO this seems like the appropriate place to
	// setup/configure bonding

	// Only bring the link up if needed
	switch msg.Attributes.OperationalState {
	case rtnetlink.OperStateUp:
	case rtnetlink.OperStateUnknown:
	default:
		err = conn.Link.Set(&rtnetlink.LinkMessage{
			Family: msg.Family,
			Type:   msg.Type,
			Index:  uint32(iface.Index),
			Flags:  unix.IFF_UP,
			Change: unix.IFF_UP,
		})

		if err != nil {
			log.Println("failed ifup")
			return err
		}
	}

	// Configure addressing on the interface
	if n.Addressing != nil {
		return n.Addressing.Configure(conn, iface.Index)
	}

	// Set up any defined routes
	for _, r := range n.Routes {
		if err = r.Add(conn); err != nil {
			return err
		}
	}

	log.Println("success")
	return err
}
