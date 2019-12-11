// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	networkapi "github.com/talos-systems/talos/api/network"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// interfacesCmd represents the net interfaces command
var interfacesCmd = &cobra.Command{
	Use:   "interfaces",
	Short: "List network interfaces",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		return setupClientE(func(c *client.Client) error {
			var remotePeer peer.Peer

			resp, err := c.Interfaces(globalCtx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting interfaces: %s", err)
				}

				helpers.Warning("%s", err)
			}

			return intersRender(&remotePeer, resp)
		})
	},
}

func intersRender(remotePeer *peer.Peer, resp *networkapi.InterfacesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tINDEX\tNAME\tMAC\tMTU\tADDRESS")

	defaultNode := addrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		for _, netif := range msg.Interfaces {
			for _, addr := range netif.Ipaddress {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%d\t%s\n", node, netif.Index, netif.Name, netif.Hardwareaddr, netif.Mtu, addr)
			}
		}
	}

	return w.Flush()
}

func init() {
	rootCmd.AddCommand(interfacesCmd)
}
