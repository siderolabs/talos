// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	clientOnly   bool
	shortVersion bool
)

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Client:")
		if shortVersion {
			version.PrintShortVersion()
		} else {
			version.PrintLongVersion()
		}

		// Exit early if we're only looking for client version
		if clientOnly {
			return nil
		}

		fmt.Println("Server:")

		return WithClient(func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer

			resp, err := c.Version(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting version: %s", err)
				}
				cli.Warning("%s", err)
			}

			defaultNode := client.AddrFromPeer(&remotePeer)

			for _, msg := range resp.Messages {
				node := defaultNode

				if msg.Metadata != nil {
					node = msg.Metadata.Hostname
				}

				fmt.Printf("\t%s:        %s\n", "NODE", node)

				version.PrintLongVersionFromExisting(msg.Version)
			}

			return nil
		})
	},
}

func init() {
	versionCmd.Flags().BoolVar(&shortVersion, "short", false, "Print the short version")
	versionCmd.Flags().BoolVar(&clientOnly, "client", false, "Print client version only")

	addCommand(versionCmd)
}
