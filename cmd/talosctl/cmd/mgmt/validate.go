// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
)

var (
	validateConfigArg string
	validateModeArg   string
	validateStrictArg bool
)

// validateCmd reads in a userData file and attempts to parse it.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := configloader.NewFromFile(validateConfigArg)
		if err != nil {
			return err
		}

		mode, err := runtime.ParseMode(validateModeArg)
		if err != nil {
			return err
		}

		opts := []config.ValidationOption{config.WithLocal()}
		if validateStrictArg {
			opts = append(opts, config.WithStrict())
		}

		warnings, err := cfg.Validate(mode, opts...)
		for _, w := range warnings {
			cli.Warning("%s", w)
		}
		if err != nil {
			return err
		}

		fmt.Printf("%s is valid for %s mode\n", validateConfigArg, validateModeArg)

		return nil
	},
}

func init() {
	validateCmd.Flags().StringVarP(&validateConfigArg, "config", "c", "", "the path of the config file")
	validateCmd.Flags().StringVarP(
		&validateModeArg,
		"mode",
		"m",
		"",
		fmt.Sprintf("the mode to validate the config for (valid values are %s, %s, and %s)", runtime.ModeMetal.String(), runtime.ModeCloud.String(), runtime.ModeContainer.String()),
	)
	cli.Should(validateCmd.MarkFlagRequired("mode"))
	validateCmd.Flags().BoolVarP(&validateStrictArg, "strict", "", false, "treat validation warnings as errors")
	addCommand(validateCmd)
}
