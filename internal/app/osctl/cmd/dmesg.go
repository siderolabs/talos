/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/talos/internal/pkg/constants"

	"github.com/autonomy/talos/internal/app/osctl/internal/client"
	"github.com/spf13/cobra"
)

// dmesgCmd represents the dmesg command
var dmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			if err := cmd.Usage(); err != nil {
				// TODO: How should we handle this?
				os.Exit(1)
			}
			os.Exit(1)
		}
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
		if err := c.Dmesg(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	dmesgCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(dmesgCmd)
}
