// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

var (
	validateConfigArg string
	validateModeArg   string
	validateStrictArg bool
)

// ValidateCmd reads in a userData file and attempts to parse it.
var ValidateCmd = &cobra.Command{
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

		opts := []validation.Option{validation.WithLocal()}
		if validateStrictArg {
			opts = append(opts, validation.WithStrict())
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
	ValidateCmd.Flags().StringVarP(&validateConfigArg, "config", "c", "", "the path of the config file")
	ValidateCmd.Flags().StringVarP(
		&validateModeArg,
		"mode",
		"m",
		"",
		fmt.Sprintf("the mode to validate the config for (valid values are %s, %s, and %s)", runtime.ModeMetal.String(), runtime.ModeCloud.String(), runtime.ModeContainer.String()),
	)
	cli.Should(ValidateCmd.MarkFlagRequired("mode"))
	ValidateCmd.Flags().BoolVarP(&validateStrictArg, "strict", "", false, "treat validation warnings as errors")
	addCommand(ValidateCmd)
}
