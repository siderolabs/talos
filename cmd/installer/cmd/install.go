// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/version"
)

// installCmd represents the install command.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if err := runInstallCmd(); err != nil {
			if err = (err); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstallCmd() (err error) {
	log.Printf("running Talos installer %s", version.NewVersion().Tag)

	seq := runtime.SequenceInstall

	if options.Upgrade {
		seq = runtime.SequenceUpgrade
	}

	p, err := platform.NewPlatform(options.Platform)
	if err != nil {
		return err
	}

	config, err := configloader.NewFromStdin()
	if err != nil {
		if errors.Is(err, configloader.ErrNoConfig) {
			log.Printf("machine configuration missing, skipping validation")
		} else {
			return fmt.Errorf("error loading machine configuration: %w", err)
		}
	} else {
		var warnings []string

		warnings, err = config.Validate(p.Mode())
		if err != nil {
			return fmt.Errorf("machine configuration is invalid: %w", err)
		}

		if len(warnings) > 0 {
			log.Printf("WARNING: config validation:")

			for _, warning := range warnings {
				log.Printf("  %s", warning)
			}
		}

		if config.Machine().Install().LegacyBIOSSupport() {
			options.LegacyBIOSSupport = true
		}

		options.EphemeralSize = config.Machine().Install().EphemeralSize()
	}

	return install.Install(p, seq, options)
}
