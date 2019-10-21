/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var upgradeImage string

// upgradeCmd represents the processes command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Talos on the target node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		upgrade()
	},
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeImage, "image", "u", "", "the container image to use for performing the install")
	rootCmd.AddCommand(upgradeCmd)
}

func upgrade() {
	var (
		err   error
		reply *machineapi.UpgradeReply
	)

	setupClient(func(c *client.Client) {
		// TODO: See if we can validate version and prevent starting upgrades to
		// an unknown version
		reply, err = c.Upgrade(globalCtx, upgradeImage)
	})

	if err != nil {
		helpers.Fatalf("error performing upgrade: %s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tACK\tSTARTED")

	for _, resp := range reply.Response {
		node := ""

		if resp.Metadata != nil {
			node = resp.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t", node, resp.Ack, time.Now())
	}

	helpers.Should(w.Flush())
}
