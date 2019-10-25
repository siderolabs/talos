/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/version"
)

var shortVersion bool

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

		fmt.Println("Server:")
		setupClient(func(c *client.Client) {
			reply, err := c.Version(globalCtx)
			if err != nil {
				helpers.Fatalf("error getting version: %s", err)
			}
			for _, resp := range reply.Response {
				node := ""

				if resp.Metadata != nil {
					node = resp.Metadata.Hostname
				}

				fmt.Printf("\t%s:        %s\n", "NODE", node)
				version.PrintLongVersionFromExisting(resp.Version)
			}
		})
	},
}

func init() {
	versionCmd.Flags().BoolVar(&shortVersion, "short", false, "Print the short version")
	rootCmd.AddCommand(versionCmd)
}
