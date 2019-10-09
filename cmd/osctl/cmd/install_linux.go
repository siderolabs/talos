/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/platform"
	machineconfig "github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	bootloader      bool
	disk            string
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
			config machineconfig.Configurator
			err    error
		)

		platform, err := platform.NewPlatform()
		if err == nil {
			if !strings.EqualFold(platform.Name(), platformArg) {
				log.Printf("platform mismatch (%s != %s)", platform.Name(), platformArg)
			} else {
				var b []byte
				b, err = platform.Configuration()
				if err != nil {
					log.Fatal(err)
				}
				var content machineconfig.Content
				content, err = machineconfig.FromBytes(b)
				if err != nil {
					log.Fatal(err)
				}
				config, err = machineconfig.New(content)
				if err != nil {
					log.Fatal(err)
				}
				extraKernelArgs = append(extraKernelArgs, config.Machine().Install().ExtraKernelArgs()...)
			}
		}

		if config == nil {
			log.Printf("failed to source config from platform; falling back to defaults")
			config = &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Version: constants.DefaultKubernetesVersion,
					},
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
		}

		cmdline := kernel.NewCmdline("")
		cmdline.Append("initrd", filepath.Join("/", "default", constants.InitramfsAsset))
		cmdline.Append(constants.KernelParamPlatform, platformArg)
		cmdline.Append(constants.KernelParamConfig, endpoint)
		if err = cmdline.AppendAll(config.Machine().Install().ExtraKernelArgs()); err != nil {
			log.Fatal(err)
		}
		cmdline.AppendDefaults()

		i, err := installer.NewInstaller(cmdline, config.Machine().Install())
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
	installCmd.Flags().BoolVar(&bootloader, "bootloader", true, "Install a booloader to the specified disk")
	installCmd.Flags().StringVar(&disk, "disk", "", "The path to the disk to install to")
	installCmd.Flags().StringVar(&endpoint, "config", "", "The value of "+constants.KernelParamConfig)
	installCmd.Flags().StringVar(&platformArg, "platform", "", "The value of "+constants.KernelParamPlatform)
	installCmd.Flags().StringArrayVar(&extraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.AddCommand(installCmd)
}
