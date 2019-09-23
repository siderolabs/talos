/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"log"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	bootloader      bool
	device          string
	endpoint        string
	platformArg     string
	extraKernelArgs []string
)

// installCmd reads in a userData file and attempts to parse it
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Talos to a specified disk",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			data *userdata.UserData
			err  error
		)

		platform, err := platform.NewPlatform()
		if err == nil {
			data, err = platform.UserData()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			// Ignore userdata load errors, since it need not necessarily exist yet.
			log.Printf("failed to source userdata from platform; falling back to defaults: %v", err)
			data = &userdata.UserData{
				Install: &userdata.Install{},
			}
		}

		// If this command is being called, it _is_ a force-install.
		data.Install.Force = true

		// Since the default for bootloader is "true," if the flag is ever false, it should override the userdata.  Thus we should always override userdata value of bootloader.
		data.Install.Bootloader = bootloader

		if len(extraKernelArgs) > 0 {
			data.Install.ExtraKernelArgs = append(data.Install.ExtraKernelArgs, extraKernelArgs...)
		}
		if device != "" {
			data.Install.Disk = device
		}

		cmdline := kernel.NewCmdline("")
		cmdline.Append("initrd", filepath.Join("/", "default", constants.InitramfsAsset))
		cmdline.Append(constants.KernelParamPlatform, platformArg)
		cmdline.Append(constants.KernelParamUserData, endpoint)
		if err = cmdline.AppendAll(data.Install.ExtraKernelArgs); err != nil {
			log.Fatal(err)
		}
		cmdline.AppendDefaults()

		i, err := installer.NewInstaller(cmdline, data)
		if err != nil {
			log.Fatal(err)
		}
		if err = i.Install(); err != nil {
			log.Fatal(err)
		}

		log.Printf("Talos (%s) installation complete", version.Tag)
	},
}

func init() {
	installCmd.Flags().BoolVar(&bootloader, "bootloader", true, "Install a booloader to the specified device")
	installCmd.Flags().StringVar(&device, "device", "", "The path to the device to install to")
	installCmd.Flags().StringVar(&endpoint, "userdata", "", "The value of "+constants.KernelParamUserData)
	installCmd.Flags().StringVar(&platformArg, "platform", "", "The value of "+constants.KernelParamPlatform)
	installCmd.Flags().StringArrayVar(&extraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.AddCommand(installCmd)
}
