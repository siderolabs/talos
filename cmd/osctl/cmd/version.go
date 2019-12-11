// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	clientOnly   bool
	shortVersion bool
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		fmt.Println("Client:")
		if shortVersion {
			version.PrintShortVersion()
		} else {
			version.PrintLongVersion()
		}

		// Exit early if we're only looking for client version
		if clientOnly {
			os.Exit(0)
		}

		fmt.Println("Server:")
		setupClient(func(c *client.Client) {
			var remotePeer peer.Peer

			resp, err := c.Version(globalCtx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					helpers.Fatalf("error getting version: %s", err)
				}
				helpers.Warning("%s", err)
			}

			defaultNode := addrFromPeer(&remotePeer)

			for _, msg := range resp.Messages {
				node := defaultNode

				if msg.Metadata != nil {
					node = msg.Metadata.Hostname
				}

				fmt.Printf("\t%s:        %s\n", "NODE", node)

				version.PrintLongVersionFromExisting(msg.Version)
			}
		})
	},
}

func init() {
	versionCmd.Flags().BoolVar(&shortVersion, "short", false, "Print the short version")
	versionCmd.Flags().BoolVar(&clientOnly, "client", false, "Print client version only")

	rootCmd.AddCommand(versionCmd)
}
