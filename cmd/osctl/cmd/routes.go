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

	networkapi "github.com/talos-systems/talos/api/network"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// routesCmd represents the net routes command
var routesCmd = &cobra.Command{
	Use:   "routes",
	Short: "List network routes",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			reply, err := c.Routes(globalCtx)
			if err != nil {
				helpers.Fatalf("error getting routes: %s", err)
			}

			routesRender(reply)
		})
	},
}

func routesRender(reply *networkapi.RoutesReply) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tINTERFACE\tDESTINATION\tGATEWAY\tMETRIC")

	for _, resp := range reply.Response {
		var node string

		if resp.Metadata != nil {
			node = resp.Metadata.Hostname
		}

		for _, route := range resp.Routes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", node, route.Interface, route.Destination, route.Gateway, route.Metric)
		}
	}

	helpers.Should(w.Flush())
}

func init() {
	rootCmd.AddCommand(routesCmd)
}
