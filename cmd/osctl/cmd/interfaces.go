/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
)

// interfacesCmd represents the net interfaces command
var interfacesCmd = &cobra.Command{
	Use:   "interfaces",
	Short: "List network interfaces",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			reply, err := c.Interfaces(globalCtx)
			if err != nil {
				helpers.Fatalf("error getting interfaces: %s", err)
			}

			intersRender(reply)
		})
	},
}

func intersRender(reply *proto.InterfacesReply) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "INDEX\tNAME\tMAC\tMTU\tADDRESS")
	for _, r := range reply.Interfaces {
		for _, addr := range r.Ipaddress {
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\n", r.Index, r.Name, r.Hardwareaddr, r.Mtu, addr)
		}
	}
	helpers.Should(w.Flush())
}

func init() {
	rootCmd.AddCommand(interfacesCmd)
}
