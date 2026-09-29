// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lldp

import (
	"fmt"
	"net"

	"github.com/siderolabs/go-lldp/pkg/receiver"
)

// NewListener opens an LLDP listener on the interface with the given kernel index.
//
// The index identifies one device instance: a link replaced under the same name gets a new index,
// so this fails rather than binding a socket to the wrong device.
func NewListener(linkIndex uint32) (Listener, error) {
	iface, err := net.InterfaceByIndex(int(linkIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to look up link %d for LLDP: %w", linkIndex, err)
	}

	listener, err := receiver.Listen(iface)
	if err != nil {
		return nil, err
	}

	return listener, nil
}
