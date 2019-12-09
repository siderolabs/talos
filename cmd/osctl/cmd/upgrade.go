// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var upgradeImage string

// upgradeCmd represents the processes command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Talos on the target node",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return upgrade()
	},
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeImage, "image", "u", "", "the container image to use for performing the install")
	rootCmd.AddCommand(upgradeCmd)
}

func upgrade() error {
	var (
		err        error
		reply      *machineapi.UpgradeReply
		remotePeer peer.Peer
	)

	setupClient(func(c *client.Client) {
		// TODO: See if we can validate version and prevent starting upgrades to
		// an unknown version
		reply, err = c.Upgrade(globalCtx, upgradeImage, grpc.Peer(&remotePeer))
	})

	if err != nil {
		if reply == nil {
			return fmt.Errorf("error performing upgrade: %s", err)
		}

		helpers.Warning("%s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tACK\tSTARTED")

	defaultNode := addrFromPeer(&remotePeer)

	for _, resp := range reply.Response {
		node := defaultNode

		if resp.Metadata != nil {
			node = resp.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t", node, resp.Ack, time.Now())
	}

	return w.Flush()
}
