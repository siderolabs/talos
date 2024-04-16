// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package installer implements the installer command.
package installer

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

func setFlagsFromEnvironment() error {
	if metaEnvBase64 := os.Getenv(constants.MetaValuesEnvVar); metaEnvBase64 != "" {
		if err := options.MetaValues.Decode(metaEnvBase64); err != nil {
			return err
		}
	}

	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Set defaults for flags from the environment variables.
	if err := setFlagsFromEnvironment(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var options = &install.Options{
	Board: constants.BoardNone,
}

var (
	bootloader bool
	dummy      string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&options.ConfigSource, "config", "", "The value of "+constants.KernelParamConfig)
	rootCmd.PersistentFlags().StringVar(&options.Disk, "disk", "", "The path to the disk to install to")
	rootCmd.PersistentFlags().StringVar(&options.Platform, "platform", "", "The value of "+constants.KernelParamPlatform)
	rootCmd.PersistentFlags().StringVar(&options.Arch, "arch", runtime.GOARCH, "The target architecture")
	rootCmd.PersistentFlags().StringVar(&dummy, "board", constants.BoardNone, "Deprecated: no op")
	rootCmd.PersistentFlags().StringArrayVar(&options.ExtraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.PersistentFlags().BoolVar(&bootloader, "bootloader", true, "Deprecated: no op")
	rootCmd.PersistentFlags().BoolVar(&options.Upgrade, "upgrade", false, "Indicates that the install is being performed by an upgrade")
	rootCmd.PersistentFlags().BoolVar(&options.Force, "force", false, "Indicates that the install should forcefully format the partition")
	rootCmd.PersistentFlags().BoolVar(&options.Zero, "zero", false, "Indicates that the install should write zeros to the disk before installing")
	rootCmd.PersistentFlags().Var(&options.MetaValues, "meta", "A key/value pair for META")
}
