// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cmd implements the installer command.
package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "installer",
	Short: "",
	Long:  ``,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var options = &install.Options{}

func init() {
	rootCmd.PersistentFlags().StringVar(&options.ConfigSource, "config", "", "The value of "+constants.KernelParamConfig)
	rootCmd.PersistentFlags().StringVar(&options.Disk, "disk", "", "The path to the disk to install to")
	rootCmd.PersistentFlags().StringVar(&options.EphemeralSize, "ephemeral-size", "", "Size for ephemeral disk partition")
	rootCmd.PersistentFlags().StringVar(&options.Platform, "platform", "", "The value of "+constants.KernelParamPlatform)
	rootCmd.PersistentFlags().StringVar(&options.Arch, "arch", runtime.GOARCH, "The target architecture")
	rootCmd.PersistentFlags().StringVar(&options.Board, "board", constants.BoardNone, "The value of "+constants.KernelParamBoard)
	rootCmd.PersistentFlags().StringArrayVar(&options.ExtraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.PersistentFlags().BoolVar(&options.Bootloader, "bootloader", true, "Install a booloader to the specified disk")
	rootCmd.PersistentFlags().BoolVar(&options.Upgrade, "upgrade", false, "Indicates that the install is being performed by an upgrade")
	rootCmd.PersistentFlags().BoolVar(&options.Force, "force", false, "Indicates that the install should forcefully format the partition")
	rootCmd.PersistentFlags().BoolVar(&options.Zero, "zero", false, "Indicates that the install should write zeros to the disk before installing")
}
