// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"bufio"
	"log"
	"net"
	"net/netip"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var jsonLogsLaunchCmdFlags struct {
	addr string
}

// jsonLogsLaunchCmd represents the kms-launch command.
var jsonLogsLaunchCmd = &cobra.Command{
	Use:    "json-logs-launch",
	Short:  "Internal command used by QEMU provisioner",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		lis, err := net.Listen("tcp", jsonLogsLaunchCmdFlags.addr)
		if err != nil {
			return err
		}

		log.Printf("starting JSON logs server on %s", jsonLogsLaunchCmdFlags.addr)

		eg, ctx := errgroup.WithContext(cmd.Context())

		eg.Go(func() error {
			for {
				conn, err := lis.Accept()
				if err != nil {
					return err
				}

				go func() {
					defer conn.Close() //nolint:errcheck

					remoteAddr := conn.RemoteAddr().String()

					if addr, err := netip.ParseAddrPort(remoteAddr); err == nil {
						remoteAddr = addr.Addr().String()
					}

					scanner := bufio.NewScanner(conn)

					for scanner.Scan() {
						log.Printf("%s: %s", remoteAddr, scanner.Text())
					}
				}()
			}
		})

		eg.Go(func() error {
			<-ctx.Done()

			return lis.Close()
		})

		return eg.Wait()
	},
}

func init() {
	jsonLogsLaunchCmd.Flags().StringVar(&jsonLogsLaunchCmdFlags.addr, "addr", "localhost:3000", "JSON logs listen address")
	addCommand(jsonLogsLaunchCmd)
}
