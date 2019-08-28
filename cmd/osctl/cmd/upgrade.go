/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var (
	image string
)

// upgradeCmd represents the processes command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Talos on the target node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if err = upgrade(); err != nil {
			helpers.Fatalf("error upgrading host: %s", err)
		}
	},
}

func init() {
	upgradeCmd.Flags().StringVarP(&image, "image", "u", "", "the container image to use for performing the install")
	rootCmd.AddCommand(upgradeCmd)
}

func upgrade() error {
	var (
		err error
		ack string
	)

	setupClient(func(c *client.Client) {
		// TODO: See if we can validate version and prevent starting upgrades to
		// an unknown version
		ack, err = c.Upgrade(globalCtx, image)
	})

	if err == nil {
		fmt.Println(ack)
	}

	return err
}
