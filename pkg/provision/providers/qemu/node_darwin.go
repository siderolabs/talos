// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"net"

	"github.com/siderolabs/talos/pkg/provision"
)

// findAPIBindAddrs returns the 0.0.0.0 address to bind to all interfaces on macos with a random port on macos.
// The bridge interface address is not used as the bridge is not yet created at this stage.
func (p *provisioner) findAPIBindAddrs(_ provision.ClusterRequest) (*net.TCPAddr, error) {
	l, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", "0"))
	if err != nil {
		return nil, err
	}

	return l.Addr().(*net.TCPAddr), l.Close()
}
