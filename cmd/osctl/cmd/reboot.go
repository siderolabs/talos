// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// rebootCmd represents the reboot command
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			if err := c.Reboot(globalCtx); err != nil {
				helpers.Fatalf("error executing reboot: %s", err)
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(rebootCmd)
}
