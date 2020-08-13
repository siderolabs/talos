// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	networkapi "github.com/talos-systems/talos/pkg/machinery/api/network"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// routesCmd represents the net routes command.
var routesCmd = &cobra.Command{
	Use:     "routes",
	Aliases: []string{"route"},
	Short:   "List network routes",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer
			resp, err := c.Routes(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting routes: %w", err)
				}
				cli.Warning("%s", err)
			}

			return routesRender(&remotePeer, resp)
		})
	},
}

func routesRender(remotePeer *peer.Peer, resp *networkapi.RoutesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tINTERFACE\tDESTINATION\tGATEWAY\tMETRIC")

	defaultNode := client.AddrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		for _, route := range msg.Routes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", node, route.Interface, route.Destination, route.Gateway, route.Metric)
		}
	}

	return w.Flush()
}

func init() {
	addCommand(routesCmd)
}
