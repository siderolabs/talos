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

// routesCmd represents the net routes command
var routesCmd = &cobra.Command{
	Use:   "routes",
	Short: "List network routes",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		return setupClientE(func(c *client.Client) error {
			var remotePeer peer.Peer
			reply, err := c.Routes(globalCtx, grpc.Peer(&remotePeer))
			if err != nil {
				if reply == nil {
					return fmt.Errorf("error getting routes: %w", err)
				}
				helpers.Warning("%s", err)
			}

			return routesRender(&remotePeer, reply)
		})
	},
}

func routesRender(remotePeer *peer.Peer, reply *networkapi.RoutesReply) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tINTERFACE\tDESTINATION\tGATEWAY\tMETRIC")

	defaultNode := addrFromPeer(remotePeer)

	for _, resp := range reply.Response {
		node := defaultNode

		if resp.Metadata != nil {
			node = resp.Metadata.Hostname
		}

		for _, route := range resp.Routes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", node, route.Interface, route.Destination, route.Gateway, route.Metric)
		}
	}

	return w.Flush()
}

func init() {
	rootCmd.AddCommand(routesCmd)
}
