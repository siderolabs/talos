// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imager implements the imager command.
package imager

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/dustin/go-humanize"
	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/pkg/archiver"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/reporter"
)

var cmdFlags struct {
	Platform              string
	Arch                  string
	Board                 string
	ImageDiskSize         string
	ExtraKernelArgs       []string
	MetaValues            install.MetaValues
	SystemExtensionImages []string
	BaseInstallerImage    string
	OutputPath            string
	OutputKind            string
	TarToStdout           bool
	OverlayName           string
	OverlayImage          string
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          "imager <profile>|-",
	Short:        "Generate various boot assets and images.",
	Long:         ``,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(context.Background(), func(ctx context.Context) error {
			report := reporter.New()
			report.Report(reporter.Update{
				Message: "assembling the finalized profile...",
				Status:  reporter.StatusRunning,
			})

			baseProfile := args[0]

			var prof profile.Profile

			if baseProfile == "-" {
				if err := yaml.NewDecoder(os.Stdin).Decode(&prof); err != nil {
					return err
				}
			} else {
				prof = profile.Profile{
					BaseProfileName: baseProfile,
					Arch:            cmdFlags.Arch,
					Board:           cmdFlags.Board,
					Platform:        cmdFlags.Platform,
					Customization: profile.CustomizationProfile{
						ExtraKernelArgs: cmdFlags.ExtraKernelArgs,
						MetaContents:    cmdFlags.MetaValues.GetMetaValues(),
					},
				}

				if cmdFlags.OverlayName != "" || cmdFlags.OverlayImage != "" {
					prof.Overlay = &profile.OverlayOptions{
						Name: cmdFlags.OverlayName,
						Image: profile.ContainerAsset{
							ImageRef: cmdFlags.OverlayImage,
						},
					}
				}

				prof.Input.SystemExtensions = xslices.Map(
					cmdFlags.SystemExtensionImages,
					func(imageRef string) profile.ContainerAsset {
						return profile.ContainerAsset{
							ImageRef: imageRef,
						}
					},
				)

				if cmdFlags.OutputKind != "" {
					outKind, err := profile.OutputKindString(cmdFlags.OutputKind)
					if err != nil {
						return err
					}

					prof.Output.Kind = outKind
				}

				if cmdFlags.BaseInstallerImage != "" {
					prof.Input.BaseInstaller = profile.ContainerAsset{
						ImageRef: cmdFlags.BaseInstallerImage,
					}
				}

				if cmdFlags.ImageDiskSize != "" {
					size, err := humanize.ParseBytes(cmdFlags.ImageDiskSize)
					if err != nil {
						return fmt.Errorf("error parsing disk image size: %w", err)
					}

					if size < profile.MinRAWDiskSize {
						return fmt.Errorf("disk image size must be at least %s", humanize.Bytes(profile.MinRAWDiskSize))
					}

					if prof.Output.ImageOptions == nil {
						prof.Output.ImageOptions = &profile.ImageOptions{}
					}

					prof.Output.ImageOptions.DiskSize = int64(size)
				}
			}

			if err := os.MkdirAll(cmdFlags.OutputPath, 0o755); err != nil {
				return err
			}

			imager, err := imager.New(prof)
			if err != nil {
				return err
			}

			if _, err = imager.Execute(ctx, cmdFlags.OutputPath, report); err != nil {
				report.Report(reporter.Update{
					Message: err.Error(),
					Status:  reporter.StatusError,
				})

				return err
			}

			if cmdFlags.TarToStdout {
				return archiver.TarGz(ctx, cmdFlags.OutputPath, os.Stdout)
			}

			return nil
		})
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cmdFlags.Platform, "platform", "", "The value of "+constants.KernelParamPlatform)
	rootCmd.PersistentFlags().StringVar(&cmdFlags.Arch, "arch", runtime.GOARCH, "The target architecture")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.BaseInstallerImage, "base-installer-image", "", "Base installer image to use")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.Board, "board", "", "The value of "+constants.KernelParamBoard)
	rootCmd.PersistentFlags().StringVar(&cmdFlags.ImageDiskSize, "image-disk-size", "", "Set custom disk image size (accepts human readable values, e.g. 6GiB)")
	rootCmd.PersistentFlags().StringArrayVar(&cmdFlags.ExtraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.PersistentFlags().Var(&cmdFlags.MetaValues, "meta", "A key/value pair for META")
	rootCmd.PersistentFlags().StringArrayVar(&cmdFlags.SystemExtensionImages, "system-extension-image", []string{}, "The image reference to the system extension to install")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OutputPath, "output", "/out", "The output directory path")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OutputKind, "output-kind", "", "Override output kind")
	rootCmd.PersistentFlags().BoolVar(&cmdFlags.TarToStdout, "tar-to-stdout", false, "Tar output and send to stdout")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OverlayName, "overlay-name", "", "The name of the overlay to use")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OverlayImage, "overlay-image", "", "The image reference to the overlay")
	rootCmd.MarkFlagsMutuallyExclusive("board", "overlay-name")
	rootCmd.MarkFlagsMutuallyExclusive("board", "overlay-image")
}
