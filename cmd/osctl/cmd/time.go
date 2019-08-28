/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/app/ntpd/proto"
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
				helpers.Fatalf("failed to parse check flag: %v", err)
			}

			var output *proto.TimeReply
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

			fmt.Printf("NTP Server: %s\n", output.Server)
			fmt.Printf("Local time: %s\n", localtime)
			fmt.Printf("Remote time: %s\n", remotetime)
		})
	},
}

func init() {
	timeCmd.Flags().StringP("check", "c", "pool.ntp.org", "checks server time against specified ntp server")
	rootCmd.AddCommand(timeCmd)
}
