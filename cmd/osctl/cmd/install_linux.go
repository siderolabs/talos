/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"log"
	"path/filepath"

	"github.com/spf13/cobra"

	localud "github.com/talos-systems/talos/cmd/osctl/internal/userdata"
	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	bootloader      bool
	device          string
	endpoint        string
	platform        string
	extraKernelArgs []string
)

// installCmd reads in a userData file and attempts to parse it
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Talos to a specified disk",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		var err error

		ud, err := localud.UserData(endpoint)
		if err != nil {
			// Ignore userdata load errors, since it need not necessarily exist yet
			log.Println("failed to source provided userdata; falling back to defaults")
			ud = &userdata.UserData{
				Install: &userdata.Install{},
			}
		}

		// If this command is being called, it _is_ a force-install.
		ud.Install.Force = true

		// Since the default for bootloader is "true," if the flag is ever false, it should override the userdata.  Thus we should always override userdata value of bootloader.
		ud.Install.Bootloader = bootloader

		if len(extraKernelArgs) > 0 {
			ud.Install.ExtraKernelArgs = append(ud.Install.ExtraKernelArgs, extraKernelArgs...)
		}
		if device != "" {
			ud.Install.Disk = device
		}

		cmdline := kernel.NewCmdline("")
		cmdline.Append("initrd", filepath.Join("/", "default", constants.InitramfsAsset))
		cmdline.Append(constants.KernelParamPlatform, platform)
		cmdline.Append(constants.KernelParamUserData, endpoint)
		if err = cmdline.AppendAll(ud.Install.ExtraKernelArgs); err != nil {
			log.Fatal(err)
		}
		cmdline.AppendDefaults()

		i, err := installer.NewInstaller(cmdline, ud)
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
	installCmd.Flags().StringVar(&platform, "platform", "", "The value of "+constants.KernelParamPlatform)
	installCmd.Flags().StringArrayVar(&extraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.AddCommand(installCmd)
}
