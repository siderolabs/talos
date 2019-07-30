/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"encoding/binary"
	"log"
	"net"

	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

type AddressInfo struct {
	NetworkInfo

	Family uint8
	Scope  uint8
	Index  uint32
}

// Message generates an AddressMessage for netlink communication
func (a *AddressInfo) Message() (msg *rtnetlink.AddressMessage) {
	// Create attributes for message
	attrs := rtnetlink.AddressAttributes{
		Address: a.IP,
	}

	if to4 := a.IP.To4(); to4 != nil {
		a.Family = unix.AF_INET
		a.IP = to4

		brd := make(net.IP, len(to4))
		binary.BigEndian.PutUint32(brd, binary.BigEndian.Uint32(to4)|^binary.BigEndian.Uint32(a.Net.Mask))

		attrs.Address = to4
		attrs.Broadcast = brd
		attrs.Local = to4
	} else {
		a.Family = unix.AF_INET6
	}

	ones, _ := a.Net.Mask.Size()

	// TODO: look at setting dynamic/permanent flag on address
	// TODO: look at setting valid lifetime and preferred lifetime
	// Ref for scope configuration
	// https://elixir.bootlin.com/linux/latest/source/net/ipv4/fib_semantics.c#L919
	// https://unix.stackexchange.com/questions/123084/what-is-the-interface-scope-global-vs-link-used-for
	msg = &rtnetlink.AddressMessage{
		Family:       a.Family,
		PrefixLength: uint8(ones),
		Scope:        a.Scope,
		Index:        a.Index,
		Attributes:   attrs,
	}

	return msg
}

// Add adds an address from an interface
func (a *AddressInfo) Add(conn *rtnetlink.Conn) error {
	addrMsg := a.Message()

	log.Printf("addr add msg %+v", addrMsg)
	log.Printf("addr add msg attrs %+v", addrMsg.Attributes)

	return conn.Address.New(addrMsg)
}

// Delete removes an address from an interface
func (a *AddressInfo) Delete(conn *rtnetlink.Conn) error {
	addrMsg := a.Message()

	log.Printf("addr delete msg %+v", addrMsg)
	log.Printf("addr delete msg attrs %+v", addrMsg.Attributes)

	return conn.Address.Delete(addrMsg)
}

// Exists returns the first address associated with a given interface
// index
func (a *AddressInfo) Exists(conn *rtnetlink.Conn) (bool, error) {
	al, err := conn.Address.List()
	if err != nil {
		return false, err
	}

	// See if the address is already configured
	for _, addr := range al {
		log.Printf("addr lookup %+v", addr)
		if addr.Index == a.Index {
			if a.IP.Equal(addr.Attributes.Address) {
				return true, nil
			}
		}
	}

	return false, err
}
