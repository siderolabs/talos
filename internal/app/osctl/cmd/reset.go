/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/talos/internal/app/osctl/internal/client"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/spf13/cobra"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := client.NewDefaultClientCredentials(talosconfig)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if target != "" {
			creds.Target = target
		}
		c, err := client.NewClient(constants.OsdPort, creds)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := c.Reset(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	resetCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(resetCmd)
}
