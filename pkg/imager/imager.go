// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imager contains code related to generation of different boot assets for Talos Linux.
package imager

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/pkg/secureboot/uki"
	"github.com/siderolabs/talos/pkg/imager/extensions"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/version"
)

// Imager is an interface for image generation.
type Imager struct {
	prof profile.Profile

	tempDir string

	// boot assets
	initramfsPath string
	cmdline       string

	sdBootPath string
	ukiPath    string
}

// New creates a new Imager.
func New(prof profile.Profile) (*Imager, error) {
	// resolve the profile if it contains a base name
	if prof.BaseProfileName != "" {
		baseProfile, ok := profile.Default[prof.BaseProfileName]
		if !ok {
			return nil, fmt.Errorf("unknown base profile: %s", prof.BaseProfileName)
		}

		baseProfile = baseProfile.DeepCopy()

		// merge the profiles
		if err := merge.Merge(&baseProfile, &prof); err != nil {
			return nil, err
		}

		prof = baseProfile
		prof.BaseProfileName = ""
	}

	if prof.Version == "" {
		prof.Version = version.Tag
	}

	if err := prof.Validate(); err != nil {
		return nil, fmt.Errorf("profile is invalid: %w", err)
	}

	prof.Input.FillDefaults(prof.Arch, prof.Version, prof.SecureBootEnabled())

	return &Imager{
		prof: prof,
	}, nil
}

// Execute image generation.
//
//nolint:gocyclo,cyclop
func (i *Imager) Execute(ctx context.Context, outputPath string) error {
	var err error

	i.tempDir, err = os.MkdirTemp("", "imager")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}

	defer os.RemoveAll(i.tempDir) //nolint:errcheck

	// 0. Dump the profile.
	if err = i.prof.Dump(os.Stderr); err != nil {
		return err
	}

	// 1. Transform `initramfs.xz` with system extensions
	if err = i.buildInitramfs(ctx); err != nil {
		return err
	}

	// 2. Prepare kernel arguments.
	if err = i.buildCmdline(); err != nil {
		return err
	}

	log.Printf("assembled kernel command line: %s", i.cmdline)

	// 3. Build UKI if Secure Boot is enabled.
	if i.prof.SecureBootEnabled() {
		if err = i.buildUKI(); err != nil {
			return err
		}
	}

	// 4. Build the output.
	outputAssetPath := filepath.Join(outputPath, i.prof.OutputPath())

	log.Printf("output path: %s", outputAssetPath)

	switch i.prof.Output.Kind {
	case profile.OutKindISO:
		err = i.outISO(outputAssetPath)
	case profile.OutKindKernel:
		err = i.outKernel(outputAssetPath)
	case profile.OutKindUKI:
		err = i.outUKI(outputAssetPath)
	case profile.OutKindInitramfs:
		err = i.outInitramfs(outputAssetPath)
	case profile.OutKindImage:
		err = i.outImage(ctx, outputAssetPath)
	case profile.OutKindInstaller:
		err = i.outInstaller(ctx, outputAssetPath)
	case profile.OutKindUnknown:
		fallthrough
	default:
		return fmt.Errorf("unknown output kind: %s", i.prof.Output.Kind)
	}

	if err != nil {
		return err
	}

	// 5. Post-process the output.
	switch i.prof.Output.OutFormat {
	case profile.OutFormatRaw:
		// do nothing
		return nil
	case profile.OutFormatXZ:
		return i.postProcessXz(outputAssetPath)
	case profile.OutFormatGZ:
		return i.postProcessGz(outputAssetPath)
	case profile.OutFormatTar:
		return i.postProcessTar(outputAssetPath)
	case profile.OutFormatUnknown:
		fallthrough
	default:
		return fmt.Errorf("unknown output format: %s", i.prof.Output.OutFormat)
	}
}

// buildInitramfs transforms `initramfs.xz` with system extensions.
func (i *Imager) buildInitramfs(ctx context.Context) error {
	if len(i.prof.Input.SystemExtensions) == 0 {
		// no system extensions, happy path
		i.initramfsPath = i.prof.Input.Initramfs.Path

		return nil
	}

	// copy the initramfs to a temporary location, as it's going to be modified during the extension build process
	tempInitramfsPath := filepath.Join(i.tempDir, "initramfs.xz")

	if err := utils.CopyFiles(utils.SourceDestination(i.prof.Input.Initramfs.Path, tempInitramfsPath)); err != nil {
		return fmt.Errorf("failed to copy initramfs: %w", err)
	}

	i.initramfsPath = tempInitramfsPath

	extensionsCheckoutDir := filepath.Join(i.tempDir, "extensions")

	// pull every extension to a temporary location
	for j, ext := range i.prof.Input.SystemExtensions {
		extensionDir := filepath.Join(extensionsCheckoutDir, strconv.Itoa(j))

		if err := os.MkdirAll(extensionDir, 0o755); err != nil {
			return fmt.Errorf("failed to create extension directory: %w", err)
		}

		if err := ext.Extract(ctx, extensionDir, i.prof.Arch); err != nil {
			return err
		}
	}

	// rebuild initramfs
	builder := extensions.Builder{
		InitramfsPath:     i.initramfsPath,
		Arch:              i.prof.Arch,
		ExtensionTreePath: extensionsCheckoutDir,
		Printf:            log.Printf,
	}

	return builder.Build()
}

// buildCmdline builds the kernel command line.
func (i *Imager) buildCmdline() error {
	p, err := platform.NewPlatform(i.prof.Platform)
	if err != nil {
		return err
	}

	cmdline := procfs.NewCmdline("")

	// platform kernel args
	cmdline.Append(constants.KernelParamPlatform, p.Name())
	cmdline.SetAll(p.KernelArgs().Strings())

	// board kernel args
	if i.prof.Board != "" {
		var b runtime.Board

		b, err = board.NewBoard(i.prof.Board)
		if err != nil {
			return err
		}

		cmdline.Append(constants.KernelParamBoard, b.Name())
		cmdline.SetAll(b.KernelArgs().Strings())
	}

	// first defaults, then extra kernel args to allow extra kernel args to override defaults
	if err = cmdline.AppendAll(kernel.DefaultArgs); err != nil {
		return err
	}

	if i.prof.SecureBootEnabled() {
		if err = cmdline.AppendAll(kernel.SecureBootArgs); err != nil {
			return err
		}
	}

	// meta values can be written only to the "image" output
	if len(i.prof.Customization.MetaContents) > 0 && i.prof.Output.Kind != profile.OutKindImage {
		// pass META values as kernel talos.environment args which will be passed via the environment to the installer
		cmdline.Append(constants.KernelParamEnvironment, constants.MetaValuesEnvVar+"="+i.prof.Customization.MetaContents.Encode())
	}

	// apply customization
	if err = cmdline.AppendAll(
		i.prof.Customization.ExtraKernelArgs,
		procfs.WithOverwriteArgs("console"),
		procfs.WithOverwriteArgs(constants.KernelParamPlatform),
	); err != nil {
		return err
	}

	i.cmdline = cmdline.String()

	return nil
}

// buildUKI assembles the UKI and signs it.
func (i *Imager) buildUKI() error {
	i.sdBootPath = filepath.Join(i.tempDir, "systemd-boot.efi.signed")
	i.ukiPath = filepath.Join(i.tempDir, "vmlinuz.efi.signed")

	builder := uki.Builder{
		Arch:       i.prof.Arch,
		SdStubPath: i.prof.Input.SDStub.Path,
		SdBootPath: i.prof.Input.SDBoot.Path,
		KernelPath: i.prof.Input.Kernel.Path,
		InitrdPath: i.initramfsPath,
		Cmdline:    i.cmdline,

		SigningKeyPath:    i.prof.Input.SecureBoot.SigningKeyPath,
		SigningCertPath:   i.prof.Input.SecureBoot.SigningCertPath,
		PCRSigningKeyPath: i.prof.Input.SecureBoot.PCRSigningKeyPath,
		PCRPublicKeyPath:  i.prof.Input.SecureBoot.PCRPublicKeyPath,

		OutSdBootPath: i.sdBootPath,
		OutUKIPath:    i.ukiPath,
	}

	return builder.Build()
}
