// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"net"
	"time"

	"github.com/pin/tftp/v3"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision/providers/vm/internal/ipxe"
)

// TFTPd starts a TFTP server on the given IPs.
func TFTPd(ctx context.Context, ips []net.IP, nextHandler string) error {
	server := tftp.NewServer(ipxe.TFTPHandler(nextHandler), nil)

	server.SetTimeout(5 * time.Second)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		<-egCtx.Done()
		server.Shutdown()

		return nil
	})

	for _, ip := range ips {
		eg.Go(func() error {
			err := server.ListenAndServe(net.JoinHostPort(ip.String(), "69"))

			if egCtx.Err() != nil {
				return nil //nolint:nilerr
			}

			return err
		})
	}

	return eg.Wait()
}
