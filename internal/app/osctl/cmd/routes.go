/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"github.com/autonomy/talos/internal/app/osctl/internal/client"
	"github.com/autonomy/talos/internal/app/osctl/internal/helpers"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/spf13/cobra"
)

// routesCmd represents the net routes command
var routesCmd = &cobra.Command{
	Use:   "routes",
	Short: "List network routes",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
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

		if err := c.Routes(); err != nil {
			helpers.Fatalf("error getting routes: %s", err)
		}
	},
}

func init() {
	routesCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(routesCmd)
}
