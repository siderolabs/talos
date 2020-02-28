// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/installer/pkg"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/runtime/platform"
	machineconfig "github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

// installCmd represents the install command
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
	var config runtime.Configurator

	p, err := platform.CurrentPlatform()
	if err == nil {
		if !strings.EqualFold(p.Name(), options.Platform) {
			return fmt.Errorf("platform mismatch (%s != %s)", p.Name(), options.Platform)
		}

		var b []byte

		b, err = p.Configuration()
		if err != nil {
			return err
		}

		config, err = machineconfig.NewFromBytes(b)
		if err != nil {
			return err
		}
	}

	if config == nil {
		log.Printf("failed to source config from platform; falling back to defaults")

		config = &v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{},
			},
			MachineConfig: &v1alpha1.MachineConfig{
				MachineInstall: &v1alpha1.InstallConfig{
					InstallForce:           true,
					InstallBootloader:      options.Bootloader,
					InstallDisk:            options.Disk,
					InstallExtraKernelArgs: options.ExtraKernelArgs,
				},
			},
		}
	}

	sequence := runtime.None
	if options.Upgrade {
		sequence = runtime.Upgrade
	}

	if err = pkg.Install(p, config, sequence, options); err != nil {
		return err
	}

	return nil
}
