// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/installer/pkg"
	"github.com/talos-systems/talos/pkg/universe"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "installer",
	Short: "",
	Long:  ``,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var options = &pkg.InstallOptions{}

func init() {
	rootCmd.PersistentFlags().StringVar(&options.ConfigSource, "config", "", "The value of "+universe.KernelParamConfig)
	rootCmd.PersistentFlags().MarkHidden("config") //nolint: errcheck
	rootCmd.PersistentFlags().StringVar(&options.Disk, "disk", "", "The path to the disk to install to")
	rootCmd.PersistentFlags().StringVar(&options.Platform, "platform", "", "The value of "+universe.KernelParamPlatform)
	rootCmd.PersistentFlags().StringArrayVar(&options.ExtraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.PersistentFlags().BoolVar(&options.Bootloader, "bootloader", true, "Install a booloader to the specified disk")
	rootCmd.PersistentFlags().BoolVar(&options.Upgrade, "upgrade", false, "Indicates that the install is being performed by an upgrade")
	rootCmd.PersistentFlags().BoolVar(&options.Force, "force", false, "Indicates that the install should forcefully format the partition")
}
