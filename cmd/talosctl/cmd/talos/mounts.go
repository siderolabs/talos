// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"math"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// mountsCmd represents the mounts command.
var mountsCmd = &cobra.Command{
	Use:     "mounts",
	Aliases: []string{"mount"},
	Short:   "List mounts",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer

			resp, err := c.Mounts(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting interfaces: %s", err)
				}

				cli.Warning("%s", err)
			}

			return mountsRender(&remotePeer, resp)
		})
	},
}

func mountsRender(remotePeer *peer.Peer, resp *machineapi.MountsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tFILESYSTEM\tSIZE(GB)\tUSED(GB)\tAVAILABLE(GB)\tPERCENT USED\tMOUNTED ON")

	defaultNode := client.AddrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		for _, r := range msg.Stats {
			percentAvailable := 100.0 - 100.0*(float64(r.Available)/float64(r.Size))

			if math.IsNaN(percentAvailable) {
				continue
			}

			node := defaultNode

			if msg.Metadata != nil {
				node = msg.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%.02f\t%.02f\t%.02f\t%.02f%%\t%s\n",
				node, r.Filesystem, float64(r.Size)*1e-9, float64(r.Size-r.Available)*1e-9, float64(r.Available)*1e-9, percentAvailable, r.MountedOn)
		}
	}

	return w.Flush()
}

func init() {
	addCommand(mountsCmd)
}
