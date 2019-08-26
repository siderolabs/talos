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
		setupClient(func(c *client.Client) {
			var reply *proto.InterfacesReply
			var err error
			if len(interfaces) > 0 {
				reply, err = c.InterfaceStats(globalCtx, interfaces)
			} else {
				reply, err = c.Interfaces(globalCtx)
			}
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

	s := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if len(interfaces) > 0 {
		for _, r := range reply.Interfaces {
			fmt.Fprintln(s, "RX PACKETS\tRX BYTES\tRX ERRORS\tRX DROPPED\tTX PACKETS\tTX BYTES\tTX ERRORS\tTX DROPPED")
			fmt.Fprintf(s, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
				r.Linkstats.RXPackets, r.Linkstats.RXBytes, r.Linkstats.RXErrors, r.Linkstats.RXDropped,
				r.Linkstats.TXPackets, r.Linkstats.TXBytes, r.Linkstats.TXErrors, r.Linkstats.TXDropped)
		}
	}
	helpers.Should(w.Flush())
	helpers.Should(s.Flush())
}

func init() {
	interfacesCmd.Flags().StringSliceVarP(&interfaces, "interface", "i", []string{}, "list of interface names to display extended interface stats")
	interfacesCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(interfacesCmd)
}
