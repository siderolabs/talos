// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"log"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	bootloader      bool
	upgradeArg      bool
	disk            string
	endpoint        string
	platformArg     string
	extraKernelArgs []string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Talos to a specified disk",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			config runtime.Configurator
			err    error
		)

		config = &v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{},
			},
			MachineConfig: &v1alpha1.MachineConfig{
				MachineInstall: &v1alpha1.InstallConfig{
					InstallForce:           true,
					InstallBootloader:      bootloader,
					InstallDisk:            disk,
					InstallExtraKernelArgs: extraKernelArgs,
				},
			},
		}

		cmdline := kernel.NewCmdline("")
		cmdline.Append("initrd", filepath.Join("/", "default", constants.InitramfsAsset))
		cmdline.Append(constants.KernelParamPlatform, platformArg)
		cmdline.Append(constants.KernelParamConfig, endpoint)
		if err = cmdline.AppendAll(config.Machine().Install().ExtraKernelArgs()); err != nil {
			return err
		}
		cmdline.AppendDefaults()

		i, err := installer.NewInstaller(cmdline, config.Machine().Install())
		if err != nil {
			return err
		}

		sequence := runtime.None
		if upgradeArg {
			sequence = runtime.Upgrade
		}

		if err = i.Install(sequence); err != nil {
			return err
		}

		log.Printf("Talos (%s) installation complete", version.Tag)

		return nil
	},
}

func init() {
	installCmd.Flags().BoolVar(&bootloader, "bootloader", true, "Install a booloader to the specified disk")
	installCmd.Flags().StringVar(&disk, "disk", "", "The path to the disk to install to")
	installCmd.Flags().StringVar(&endpoint, "config", "", "The value of "+constants.KernelParamConfig)
	installCmd.Flags().StringVar(&platformArg, "platform", "", "The value of "+constants.KernelParamPlatform)
	installCmd.Flags().BoolVar(&upgradeArg, "upgrade", false, "Indicates that the install is being performed by an upgrade")
	installCmd.Flags().StringArrayVar(&extraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.AddCommand(installCmd)
}
