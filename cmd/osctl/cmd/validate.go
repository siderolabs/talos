/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
)

var (
	validateConfigArg string
	validateModeArg   string
)

// validateCmd reads in a userData file and attempts to parse it
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := config.FromFile(validateConfigArg)
		if err != nil {
			log.Fatal(err)
		}
		config, err := config.New(content)
		if err != nil {
			log.Fatal(err)
		}

		mode, err := runtime.ModeFromString(validateModeArg)
		if err != nil {
			log.Fatal(err)
		}
		if err := config.Validate(mode); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%s is valid for %s mode", validateConfigArg, validateModeArg)
	},
}

func init() {
	validateCmd.Flags().StringVarP(&validateConfigArg, "config", "c", "", "the path of the config file")
	validateCmd.Flags().StringVarP(&validateModeArg, "mode", "m", "", "the mode to validate the config for")
	rootCmd.AddCommand(validateCmd)
}
