// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"log"
	"net"

	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// setMTU sets the link MTU
func (n *Networkd) setMTU(idx int, mtu uint32) error {
	msg, err := n.NlConn.Link.Get(uint32(idx))
	if err != nil {
		log.Printf("failed to get link %d\n", idx)
		return err
	}

	err = n.NlConn.Link.Set(&rtnetlink.LinkMessage{
		Family: msg.Family,
		Type:   msg.Type,
		Index:  uint32(idx),
		Flags:  msg.Flags,
		Change: 0,
		Attributes: &rtnetlink.LinkAttributes{
			MTU: mtu,
		},
	})

	return err
}

// nolint: unused
func (n *Networkd) createBond(name string) (*net.Interface, error) {
	err := n.NlConn.Link.New(&rtnetlink.LinkMessage{
		Family: unix.AF_UNSPEC,
		Type:   0,
		Attributes: &rtnetlink.LinkAttributes{
			Name: name,
			Info: &rtnetlink.LinkInfo{
				Kind: "bond",
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	return net.InterfaceByName(name)
}

// nolint: unused
func (n *Networkd) configureBond(idx int, attrs []netlink.Attribute) error {
	// Request the details of the interface
	msg, err := n.NlConn.Link.Get(uint32(idx))
	if err != nil {
		return err
	}

	// We could probably handle all these in a single pass; guess we can leave it
	// up for debate on 1 attribute at a time or all at once
	for _, attr := range attrs {
		nlAttrBytes, err := netlink.MarshalAttributes([]netlink.Attribute{attr})
		if err != nil {
			return err
		}

		err = n.NlConn.Link.Set(&rtnetlink.LinkMessage{
			Family: unix.AF_UNSPEC,
			Type:   msg.Type,
			Index:  uint32(idx),
			Flags:  0,
			Change: 0,
			Attributes: &rtnetlink.LinkAttributes{
				Info: &rtnetlink.LinkInfo{
					// https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/if_link.h#L612
					Kind: "bond",
					Data: nlAttrBytes,
				},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// nolint: unused
func (n *Networkd) enslaveLink(bondIndex *uint32, links ...*net.Interface) error {
	// Set the interface operationally UP
	for _, iface := range links {
		// Request the details of the interface
		msg, err := n.NlConn.Link.Get(uint32(iface.Index))
		if err != nil {
			return err
		}

		// rtnl.Down
		// TODO is this really needed(?)
		err = n.NlConn.Link.Set(&rtnetlink.LinkMessage{
			Family: msg.Family,
			Type:   msg.Type,
			Index:  uint32(iface.Index),
			Flags:  0,
			Change: unix.IFF_UP,
		})
		if err != nil {
			return err
		}

		// Set link master to bond interface
		err = n.NlConn.Link.Set(&rtnetlink.LinkMessage{
			Family: msg.Family,
			Type:   msg.Type,
			Index:  uint32(iface.Index),
			Change: 0,
			Flags:  0,
			Attributes: &rtnetlink.LinkAttributes{
				Master: bondIndex,
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}
