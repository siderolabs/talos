// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	timeapi "github.com/talos-systems/talos/pkg/machinery/api/time"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var timeCmdFlags struct {
	ntpServer string
}

// timeCmd represents the time command.
var timeCmd = &cobra.Command{
	Use:   "time [--check server]",
	Short: "Gets current server time",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var (
				resp       *timeapi.TimeResponse
				remotePeer peer.Peer
				err        error
			)

			if timeCmdFlags.ntpServer == "" {
				resp, err = c.Time(ctx, grpc.Peer(&remotePeer))
			} else {
				resp, err = c.TimeCheck(ctx, timeCmdFlags.ntpServer, grpc.Peer(&remotePeer))
			}

			if err != nil {
				if resp == nil {
					return fmt.Errorf("error fetching time: %w", err)
				}

				cli.Warning("%s", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tNTP-SERVER\tNODE-TIME\tNTP-SERVER-TIME")

			defaultNode := client.AddrFromPeer(&remotePeer)

			var localtime, remotetime time.Time
			for _, msg := range resp.Messages {
				node := defaultNode

				if msg.Metadata != nil {
					node = msg.Metadata.Hostname
				}

				if !msg.Localtime.IsValid() {
					return fmt.Errorf("error parsing local time")
				}

				if !msg.Remotetime.IsValid() {
					return fmt.Errorf("error parsing remote time")
				}

				localtime = msg.Localtime.AsTime()
				remotetime = msg.Remotetime.AsTime()

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", node, msg.Server, localtime.String(), remotetime.String())
			}

			return w.Flush()
		})
	},
}

func init() {
	timeCmd.Flags().StringVarP(&timeCmdFlags.ntpServer, "check", "c", "", "checks server time against specified ntp server")
	addCommand(timeCmd)
}
