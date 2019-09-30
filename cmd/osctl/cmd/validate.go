/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/config"
)

var configFile string

// validateCmd reads in a userData file and attempts to parse it
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := config.FromFile(configFile)
		if err != nil {
			log.Fatal(err)
		}
		config, err := config.New(content)
		if err != nil {
			log.Fatal(err)
		}
		if err := config.Validate(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	validateCmd.Flags().StringVarP(&configFile, "config", "u", "", "the path of the config file")
	rootCmd.AddCommand(validateCmd)
}
