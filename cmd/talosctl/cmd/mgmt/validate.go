// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
)

var (
	validateConfigArg string
	validateModeArg   string
)

// validateCmd reads in a userData file and attempts to parse it.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := configloader.NewFromFile(validateConfigArg)
		if err != nil {
			return err
		}

		mode, err := runtime.ParseMode(validateModeArg)
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
	cli.Should(validateCmd.MarkFlagRequired("mode"))
	addCommand(validateCmd)
}
