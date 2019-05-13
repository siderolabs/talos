/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/constants"
)

var (
	assetURL string
	local    bool
)

// upgradeCmd represents the processes command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Talos on the target node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if local {
			if err = localUpgrade(); err != nil {
				helpers.Fatalf("error upgrading host: %s", err)
			}
		} else {
			if err = remoteUpgrade(); err != nil {
				helpers.Fatalf("error upgrading host: %s", err)
			}
		}

	},
}

func init() {
	upgradeCmd.Flags().BoolVarP(&local, "local", "l", false, "operate in local mode")
	upgradeCmd.Flags().StringVarP(&assetURL, "url", "u", "", "url hosting upgrade assets (excluding filename)")
	upgradeCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(upgradeCmd)
}

func remoteUpgrade() error {
	creds, err := client.NewDefaultClientCredentials(talosconfig)
	if err != nil {
		return err
	}
	if target != "" {
		creds.Target = target
	}
	c, err := client.NewClient(constants.OsdPort, creds)
	if err != nil {
		return err
	}

	// TODO: See if we can validate version and prevent
	// starting upgrades to an unknown version
	return c.Upgrade(assetURL)
}
