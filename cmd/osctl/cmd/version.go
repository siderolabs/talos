/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
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

		if shortVersion {
			version.PrintShortVersion()
		} else {
			if err := version.PrintLongVersion(); err != nil {
				helpers.Fatalf("error printing long version: %s", err)
			}
		}
		setupClient(func(c *client.Client) {
			version, err := c.Version(globalCtx)
			if err != nil {
				helpers.Fatalf("error getting version: %s", err)
			}
			_, err = os.Stdout.Write(version)
			helpers.Should(err)
		})
	},
}

func init() {
	versionCmd.Flags().BoolVar(&shortVersion, "short", false, "Print the short version")
	rootCmd.AddCommand(versionCmd)
}
