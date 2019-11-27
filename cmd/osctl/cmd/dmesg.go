// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// dmesgCmd represents the dmesg command
var dmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			md := metadata.New(make(map[string]string))
			md.Set("targets", target...)
			reply, err := c.Dmesg(metadata.NewOutgoingContext(globalCtx, md))
			if err != nil {
				helpers.Fatalf("error getting dmesg: %s", err)
			}

			for _, resp := range reply.Response {
				if len(reply.Response) > 1 {
					fmt.Println(resp.Metadata.Hostname)
				}
				_, err = os.Stdout.Write(resp.Bytes)
				helpers.Should(err)
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(dmesgCmd)
}
