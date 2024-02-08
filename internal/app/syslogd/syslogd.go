// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package syslogd provides a syslogd service that listens on a unix socket
package syslogd

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/syslogd/internal/parser"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Main is an entrypoint to the API service.
func Main(ctx context.Context, _ runtime.Runtime, logWriter io.Writer) error {
	return Run(ctx, logWriter, constants.SyslogListenSocketPath)
}

// Run starts the syslogd service.
func Run(ctx context.Context, logWriter io.Writer, listenSocketPath string) error {
	unixAddr, err := net.ResolveUnixAddr("unixgram", listenSocketPath)
	if err != nil {
		return err
	}

	connection, err := net.ListenUnixgram("unixgram", unixAddr)
	if err != nil {
		return err
	}

	if err = connection.SetReadBuffer(65536); err != nil {
		return fmt.Errorf("failed to set read buffer: %w", err)
	}

	buf := make([]byte, 1024)

	go func(con *net.UnixConn) {
		for {
			n, err := con.Read(buf)
			if err != nil {
				continue
			}

			syslogJSON, err := parser.Parse(buf[:n])
			if err != nil { // if the message is not a valid syslog message, skip it
				continue
			}

			fmt.Fprintln(logWriter, syslogJSON)
		}
	}(connection)

	<-ctx.Done()

	return connection.Close()
}
