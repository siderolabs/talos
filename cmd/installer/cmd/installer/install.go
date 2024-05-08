// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installer

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// installCmd represents the installation command.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		return runInstallCmd(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstallCmd(ctx context.Context) (err error) {
	log.Printf("running Talos installer %s", version.NewVersion().Tag)

	mode := install.ModeInstall

	if options.Upgrade {
		mode = install.ModeUpgrade
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

		if config.Machine() != nil && config.Machine().Install().LegacyBIOSSupport() {
			options.LegacyBIOSSupport = true
		}
	}

	return install.Install(ctx, p, mode, options)
}
