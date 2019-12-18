// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var upgradeImage string

// upgradeCmd represents the processes command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Talos on the target node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return upgrade()
	},
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeImage, "image", "i", "", "the container image to use for performing the install")
	rootCmd.AddCommand(upgradeCmd)
}

func upgrade() error {
	return WithClient(func(ctx context.Context, c *client.Client) error {
		var remotePeer peer.Peer

		// TODO: See if we can validate version and prevent starting upgrades to
		// an unknown version
		resp, err := c.Upgrade(ctx, upgradeImage, grpc.Peer(&remotePeer))
		if err != nil {
			if resp == nil {
				return fmt.Errorf("error performing upgrade: %s", err)
			}

			helpers.Warning("%s", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tACK\tSTARTED")

		defaultNode := helpers.AddrFromPeer(&remotePeer)

		for _, msg := range resp.Messages {
			node := defaultNode

			if msg.Metadata != nil {
				node = msg.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t", node, msg.Ack, time.Now())
		}

		return w.Flush()
	})
}
