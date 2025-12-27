// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imager implements the imager command.
package imager

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/pkg/archiver"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/overlay"
	"github.com/siderolabs/talos/pkg/reporter"
)

var cmdFlags struct {
	Platform string
	Arch     string
	// Insecure can be set to true to force pull from insecure registry.
	Insecure              bool
	ExtraKernelArgs       []string
	MetaValues            install.MetaValues
	SystemExtensionImages []string
	BaseInstallerImage    string
	ImageCache            string
	EmbeddedConfigPath    string
	OutputPath            string
	OutputKind            string
	TarToStdout           bool
	OverlayName           string
	OverlayImage          string
	OverlayOptions        []string
	// Only used when generating a secure boot iso without also providing a secure boot database.
	SecurebootIncludeWellKnownCerts bool
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

			if os.Getuid() != 0 {
				report.Report(reporter.Update{
					Message: "imager is not running as root, re-execing with a usernamespace...",
					Status:  reporter.StatusRunning,
				})

				return cli.ReExecWithUserNamespace(cmd.Context())
			}

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
					Platform:        cmdFlags.Platform,
					Customization: profile.CustomizationProfile{
						ExtraKernelArgs: cmdFlags.ExtraKernelArgs,
						MetaContents:    cmdFlags.MetaValues.GetMetaValues(),
					},
				}

				extraOverlayOptions := overlay.ExtraOptions{}

				for _, option := range cmdFlags.OverlayOptions {
					if strings.HasPrefix(option, "@") {
						data, err := os.ReadFile(option[1:])
						if err != nil {
							return err
						}

						decoder := yaml.NewDecoder(bytes.NewReader(data))
						decoder.KnownFields(true)

						if err := decoder.Decode(&extraOverlayOptions); err != nil {
							return err
						}

						continue

					}

					k, v, _ := strings.Cut(option, "=")

					if strings.HasPrefix(v, "@") {
						data, err := os.ReadFile(v[1:])
						if err != nil {
							return err
						}

						v = string(data)
					}

					extraOverlayOptions[k] = v
				}

				if cmdFlags.OverlayName != "" || cmdFlags.OverlayImage != "" {
					prof.Overlay = &profile.OverlayOptions{
						Name: cmdFlags.OverlayName,
						Image: profile.ContainerAsset{
							ImageRef: cmdFlags.OverlayImage,
						},
						ExtraOptions: extraOverlayOptions,
					}

					prof.Input.OverlayInstaller.ImageRef = cmdFlags.OverlayImage
				}

				prof.Input.SystemExtensions = xslices.Map(
					cmdFlags.SystemExtensionImages,
					func(imageRef string) profile.ContainerAsset {
						return profile.ContainerAsset{
							ImageRef:      imageRef,
							ForceInsecure: cmdFlags.Insecure,
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

				if cmdFlags.ImageCache != "" {
					parseOpts := []name.Option{name.StrictValidation}

					if cmdFlags.Insecure {
						parseOpts = append(parseOpts, name.Insecure)
					}

					if _, err := name.ParseReference(cmdFlags.ImageCache, parseOpts...); err == nil {
						prof.Input.ImageCache = profile.ContainerAsset{
							ImageRef: cmdFlags.ImageCache,
						}
					} else {
						prof.Input.ImageCache = profile.ContainerAsset{
							OCIPath: cmdFlags.ImageCache,
						}
					}
				}

				if cmdFlags.Insecure {
					prof.Input.BaseInstaller.ForceInsecure = cmdFlags.Insecure
					prof.Input.ImageCache.ForceInsecure = cmdFlags.Insecure
				}

				if cmdFlags.SecurebootIncludeWellKnownCerts {
					if prof.Input.SecureBoot == nil {
						prof.Input.SecureBoot = &profile.SecureBootAssets{}
					}
					prof.Input.SecureBoot.IncludeWellKnownCerts = true
				}

				if cmdFlags.EmbeddedConfigPath != "" {
					data, err := os.ReadFile(cmdFlags.EmbeddedConfigPath)
					if err != nil {
						return fmt.Errorf("error reading embedded config file: %w", err)
					}

					prof.Customization.EmbeddedMachineConfiguration = string(data)
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
	rootCmd.PersistentFlags().StringVar(&cmdFlags.ImageCache, "image-cache", "", "Image cache container image or oci path")
	rootCmd.PersistentFlags().BoolVar(&cmdFlags.Insecure, "insecure", false, "Pull assets from insecure registry")
	rootCmd.PersistentFlags().StringArrayVar(&cmdFlags.ExtraKernelArgs, "extra-kernel-arg", []string{}, "Extra argument to pass to the kernel")
	rootCmd.PersistentFlags().Var(&cmdFlags.MetaValues, "meta", "A key/value pair for META")
	rootCmd.PersistentFlags().StringArrayVar(&cmdFlags.SystemExtensionImages, "system-extension-image", []string{}, "The image reference to the system extension to install")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OutputPath, "output", "/out", "The output directory path")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OutputKind, "output-kind", "", "Override output kind")
	rootCmd.PersistentFlags().BoolVar(&cmdFlags.TarToStdout, "tar-to-stdout", false, "Tar output and send to stdout")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OverlayName, "overlay-name", "", "The name of the overlay to use")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.OverlayImage, "overlay-image", "", "The image reference to the overlay")
	rootCmd.PersistentFlags().StringArrayVar(&cmdFlags.OverlayOptions, "overlay-option", []string{}, "Extra options to pass to the overlay")
	rootCmd.PersistentFlags().StringVar(&cmdFlags.EmbeddedConfigPath, "embedded-config-path", "", "Path to a file containing the machine configuration to embed into the image")
	rootCmd.PersistentFlags().BoolVar(
		&cmdFlags.SecurebootIncludeWellKnownCerts, "secureboot-include-well-known-certs", false, "Include well-known (Microsoft) UEFI certificates when generating a secure boot database")
}
