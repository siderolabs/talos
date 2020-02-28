// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
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
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := config.NewFromFile(validateConfigArg)
		if err != nil {
			return err
		}

		mode, err := runtime.ModeFromString(validateModeArg)
		if err != nil {
			return err
		}
		if err := config.Validate(mode); err != nil {
			return err
		}

		fmt.Printf("%s is valid for %s mode", validateConfigArg, validateModeArg)

		return nil
	},
}

func init() {
	validateCmd.Flags().StringVarP(&validateConfigArg, "config", "c", "", "the path of the config file")
	validateCmd.Flags().StringVarP(&validateModeArg, "mode", "m", "", "the mode to validate the config for")
	helpers.Should(validateCmd.MarkFlagRequired("mode"))
	rootCmd.AddCommand(validateCmd)
}
