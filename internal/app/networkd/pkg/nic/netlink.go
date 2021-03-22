// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic

import (
	"net"

	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// createLink creates an interface.
func (n *NetworkInterface) createLink(name string, info *rtnetlink.LinkInfo) error {
	err := n.rtConn.Link.New(&rtnetlink.LinkMessage{
		Family: unix.AF_UNSPEC,
		Type:   0,
		Attributes: &rtnetlink.LinkAttributes{
			Name: name,
			Info: info,
		},
	})

	return err
}

// createLink creates an interface.
func (n *NetworkInterface) createSubLink(name string, info *rtnetlink.LinkInfo, master *uint32) error {
	err := n.rtConn.Link.New(&rtnetlink.LinkMessage{
		Family: unix.AF_UNSPEC,
		Type:   0,
		Attributes: &rtnetlink.LinkAttributes{
			Name: name,
			Info: info,
			Type: *master,
		},
	})

	return err
}

// setMTU sets the link MTU.
func (n *NetworkInterface) setMTU(idx int, mtu uint32) error {
	msg, err := n.rtConn.Link.Get(uint32(idx))
	if err != nil {
		return err
	}

	if msg.Attributes != nil && msg.Attributes.MTU == mtu {
		return nil
	}

	err = n.rtConn.Link.Set(&rtnetlink.LinkMessage{
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

func (n *NetworkInterface) configureBond(idx int, attrs *netlink.AttributeEncoder) error {
	// Request the details of the interface
	msg, err := n.rtConn.Link.Get(uint32(idx))
	if err != nil {
		return err
	}

	nlAttrBytes, err := attrs.Encode()
	if err != nil {
		return err
	}

	err = n.rtConn.Link.Set(&rtnetlink.LinkMessage{
		Family: unix.AF_UNSPEC,
		Type:   msg.Type,
		Index:  msg.Index,
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

	return nil
}

func (n *NetworkInterface) configureWireguard(name string, config *wgtypes.Config) error {
	c, err := wgctrl.New()
	if err != nil {
		return err
	}

	defer c.Close() //nolint:errcheck

	return c.ConfigureDevice(name, *config)
}

func (n *NetworkInterface) enslaveLink(bondIndex *uint32, links ...*net.Interface) error {
	// Set the interface operationally UP
	for _, iface := range links {
		// Request the details of the interface
		msg, err := n.rtConn.Link.Get(uint32(iface.Index))
		if err != nil {
			return err
		}

		// rtnl.Down
		err = n.rtConn.Link.Set(&rtnetlink.LinkMessage{
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
		err = n.rtConn.Link.Set(&rtnetlink.LinkMessage{
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
