/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"

	timeapi "github.com/talos-systems/talos/api/time"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// timeCmd represents the time command
var timeCmd = &cobra.Command{
	Use:   "time [--check server]",
	Short: "Gets current server time",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		setupClient(func(c *client.Client) {
			server, err := cmd.Flags().GetString("check")
			if err != nil {
				helpers.Fatalf("failed to parse check flag: %w", err)
			}

			var output *timeapi.TimeReply
			if server == "" {
				output, err = c.Time(globalCtx)
				if err != nil {
					helpers.Fatalf("error fetching time: %s", err)
				}
			} else {
				output, err = c.TimeCheck(globalCtx, server)
				if err != nil {
					helpers.Fatalf("error fetching time: %s", err)
				}
			}

			var localtime, remotetime time.Time
			localtime, err = ptypes.Timestamp(output.Localtime)
			if err != nil {
				helpers.Fatalf("error parsing local time: %s", err)
			}
			remotetime, err = ptypes.Timestamp(output.Remotetime)
			if err != nil {
				helpers.Fatalf("error parsing remote time: %s", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NTP-SERVER\tLOCAL-TIME\tREMOTE-TIME")
			fmt.Fprintf(w, "%s\t%s\t%s\n", output.Server, localtime.String(), remotetime.String())
			helpers.Should(w.Flush())
		})
	},
}

func init() {
	timeCmd.Flags().StringP("check", "c", "pool.ntp.org", "checks server time against specified ntp server")
	rootCmd.AddCommand(timeCmd)
}
