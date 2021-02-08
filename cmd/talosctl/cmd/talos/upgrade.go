// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var (
	upgradeImage string
	preserve     bool
	stage        bool
)

// upgradeCmd represents the processes command.
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
	upgradeCmd.Flags().BoolVarP(&preserve, "preserve", "p", false, "preserve data")
	upgradeCmd.Flags().BoolVarP(&stage, "stage", "s", false, "stage the upgrade to perform it after a reboot")
	upgradeCmd.Flags().BoolVarP(&force, "force", "f", false, "force the upgrade (skip checks on etcd health and members, might lead to data loss)")
	addCommand(upgradeCmd)
}

func upgrade() error {
	return WithClient(func(ctx context.Context, c *client.Client) error {
		var remotePeer peer.Peer

		// TODO: See if we can validate version and prevent starting upgrades to
		// an unknown version
		resp, err := c.Upgrade(ctx, upgradeImage, preserve, stage, force, grpc.Peer(&remotePeer))
		if err != nil {
			if resp == nil {
				return fmt.Errorf("error performing upgrade: %s", err)
			}

			cli.Warning("%s", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tACK\tSTARTED")

		defaultNode := client.AddrFromPeer(&remotePeer)

		for _, msg := range resp.Messages {
			node := defaultNode

			if msg.Metadata != nil {
				node = msg.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t\n", node, msg.Ack, time.Now())
		}

		return w.Flush()
	})
}
