/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/constants"
)

// dmesgCmd represents the dmesg command
var dmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}
		creds, err := client.NewDefaultClientCredentials(talosconfig)
		if err != nil {
			helpers.Fatalf("error getting client credentials: %s", err)
		}
		if target != "" {
			creds.Target = target
		}
		c, err := client.NewClient(constants.OsdPort, creds)
		if err != nil {
			helpers.Fatalf("error constructing client: %s", err)
		}
		if err := c.Dmesg(); err != nil {
			helpers.Fatalf("error getting dmesg: %s", err)
		}
	},
}

func init() {
	dmesgCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(dmesgCmd)
}
