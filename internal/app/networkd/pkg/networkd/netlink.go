/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"log"

	"github.com/jsimonetti/rtnetlink"
)

// setMTU sets the link MTU
func (n *Networkd) setMTU(idx int, mtu uint32) error {
	msg, err := n.nlConn.Link.Get(uint32(idx))
	if err != nil {
		log.Printf("failed to get link %d\n", idx)
		return err
	}

	err = n.nlConn.Link.Set(&rtnetlink.LinkMessage{
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
