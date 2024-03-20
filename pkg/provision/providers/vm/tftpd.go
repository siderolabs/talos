// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"net"
	"time"

	"github.com/pin/tftp/v3"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision/providers/vm/internal/ipxe"
)

// TFTPd starts a TFTP server on the given IPs.
func TFTPd(ips []net.IP, nextHandler string) error {
	server := tftp.NewServer(ipxe.TFTPHandler(nextHandler), nil)

	server.SetTimeout(5 * time.Second)

	var eg errgroup.Group

	for _, ip := range ips {
		eg.Go(func() error {
			return server.ListenAndServe(net.JoinHostPort(ip.String(), "69"))
		})
	}

	return eg.Wait()
}
