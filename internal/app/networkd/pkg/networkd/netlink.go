/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"log"
	"net"
	"os"
	"syscall"

	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// ifup sets the link state to up
func (n *Networkd) ifup(idx uint32) error {
	msg, err := n.Conn.Link.Get(idx)
	if err != nil {
		log.Printf("failed to get link %d\n", idx)
		return err
	}

	// Only bring the link up if needed
	switch msg.Attributes.OperationalState {
	case rtnetlink.OperStateUp:
	case rtnetlink.OperStateUnknown:
	default:
		err = n.Conn.Link.Set(&rtnetlink.LinkMessage{
			Family: msg.Family,
			Type:   msg.Type,
			Index:  idx,
			Flags:  unix.IFF_UP,
			Change: unix.IFF_UP,
		})

		if err != nil {
			log.Println("failed ifup")
			return err
		}
	}

	return err
}

// setMTU sets the link MTU
func (n *Networkd) setMTU(idx, mtu uint32) error {
	msg, err := n.Conn.Link.Get(idx)
	if err != nil {
		log.Printf("failed to get link %d\n", idx)
		return err
	}

	err = n.Conn.Link.Set(&rtnetlink.LinkMessage{
		Family: msg.Family,
		Type:   msg.Type,
		Index:  idx,
		Flags:  msg.Flags,
		Change: 0,
		Attributes: &rtnetlink.LinkAttributes{
			MTU: mtu,
		},
	})

	return err
}

// AddressAdd attempts to configure an address on an interface specified by
// the rtnetlink.AddressMessage. If the address is already configured on the
// interface, no action is taken.
func (n *Networkd) AddressAdd(msg *rtnetlink.AddressMessage) error {
	exists, err := n.addressExists(msg)
	if err != nil {
		return err
	}

	if exists {
		return err
	}

	return n.Conn.Address.New(msg)
}

func (n *Networkd) addressExists(msg *rtnetlink.AddressMessage) (bool, error) {
	al, err := n.Conn.Address.List()
	if err != nil {
		return false, err
	}

	// See if the address is already configured
	for _, addr := range al {
		if addr.Index == msg.Index {
			if !msg.Attributes.Address.Equal(addr.Attributes.Address) {
				continue
			}
			if msg.PrefixLength != addr.PrefixLength {
				continue
			}
			return true, err
		}
	}
	return false, err
}

// RouteAdd attempts to configure a route specified by the
// rtnetlink.RouteMessage. If the route is already configured in the
// routing table, no action is taken.
func (n *Networkd) RouteAdd(msg *rtnetlink.RouteMessage) error {
	exists, err := n.routeExists(msg)
	if err != nil {
		return err
	}

	if exists {
		return err
	}

	if err = n.Conn.Route.Add(msg); err != nil {
		// nolint: gocritic
		switch err := err.(type) {
		case *netlink.OpError:
			// ignore the error if it's -EEXIST or -ESRCH
			if !os.IsExist(err.Err) && err.Err != syscall.ESRCH {
				return err
			}
		}
	}

	return nil
}

func (n *Networkd) routeExists(msg *rtnetlink.RouteMessage) (bool, error) {
	rl, err := n.Conn.Route.List()
	if err != nil {
		return false, err
	}

	for _, route := range rl {
		if msg.Attributes.OutIface != route.Attributes.OutIface {
			continue
		}

		// This feels super ugly
		// Only compare against what was given
		if msg.Attributes.Dst != nil {
			if !compareNets(msg.Attributes.Dst, route.Attributes.Dst) {
				continue
			}
			if msg.DstLength != route.DstLength {
				continue
			}
		}

		if msg.Attributes.Gateway != nil {
			if !compareNets(msg.Attributes.Gateway, route.Attributes.Gateway) {
				continue
			}
		}

		// TODO
		// Unsure what to do if scope doesnt match
		// feels like it should require a delete + add

		// TODO figure out if Src is actually needed
		/*
			if msg.Attributes.Src != nil {
				if !compareNets(msg.Attributes.Src, route.Attributes.Src) {
					continue
				}
				if msg.SrcLength != route.SrcLength {
					continue
				}
			}
		*/
		return true, err

	}

	return false, err
}

// compareNets is a simple utility function to see if address `a` is
// equal to `b`
func compareNets(a, b net.IP) bool {
	if a == nil && b == nil {
		return true
	}

	if a != nil && a.Equal(b) {
		return true
	}

	return false
}
