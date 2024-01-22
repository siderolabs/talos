// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imager contains code related to generation of different boot assets for Talos Linux.
package imager

import (
	"context"
	"fmt"
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
	"github.com/siderolabs/talos/pkg/imager/quirks"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/reporter"
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
func (i *Imager) Execute(ctx context.Context, outputPath string, report *reporter.Reporter) (outputAssetPath string, err error) {
	i.tempDir, err = os.MkdirTemp("", "imager")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	defer os.RemoveAll(i.tempDir) //nolint:errcheck

	report.Report(reporter.Update{
		Message: "profile ready:",
		Status:  reporter.StatusSucceeded,
	})

	// 0. Dump the profile.
	if err = i.prof.Dump(os.Stderr); err != nil {
		return "", err
	}

	// 1. Transform `initramfs.xz` with system extensions
	if err = i.buildInitramfs(ctx, report); err != nil {
		return "", err
	}

	// 2. Prepare kernel arguments.
	if err = i.buildCmdline(); err != nil {
		return "", err
	}

	report.Report(reporter.Update{
		Message: fmt.Sprintf("kernel command line: %s", i.cmdline),
		Status:  reporter.StatusSucceeded,
	})

	// 3. Build UKI if Secure Boot is enabled.
	if i.prof.SecureBootEnabled() {
		if err = i.buildUKI(ctx, report); err != nil {
			return "", err
		}
	}

	// 4. Build the output.
	outputAssetPath = filepath.Join(outputPath, i.prof.OutputPath())

	switch i.prof.Output.Kind {
	case profile.OutKindISO:
		err = i.outISO(ctx, outputAssetPath, report)
	case profile.OutKindKernel:
		err = i.outKernel(outputAssetPath, report)
	case profile.OutKindUKI:
		err = i.outUKI(outputAssetPath, report)
	case profile.OutKindInitramfs:
		err = i.outInitramfs(outputAssetPath, report)
	case profile.OutKindCmdline:
		err = i.outCmdline(outputAssetPath)
	case profile.OutKindImage:
		err = i.outImage(ctx, outputAssetPath, report)
	case profile.OutKindInstaller:
		err = i.outInstaller(ctx, outputAssetPath, report)
	case profile.OutKindUnknown:
		fallthrough
	default:
		return "", fmt.Errorf("unknown output kind: %s", i.prof.Output.Kind)
	}

	if err != nil {
		return "", err
	}

	report.Report(reporter.Update{
		Message: fmt.Sprintf("output asset path: %s", outputAssetPath),
		Status:  reporter.StatusSucceeded,
	})

	// 5. Post-process the output.
	switch i.prof.Output.OutFormat {
	case profile.OutFormatRaw:
		// do nothing
		return outputAssetPath, nil
	case profile.OutFormatXZ:
		return i.postProcessXz(outputAssetPath, report)
	case profile.OutFormatGZ:
		return i.postProcessGz(outputAssetPath, report)
	case profile.OutFormatTar:
		return i.postProcessTar(outputAssetPath, report)
	case profile.OutFormatUnknown:
		fallthrough
	default:
		return "", fmt.Errorf("unknown output format: %s", i.prof.Output.OutFormat)
	}
}

// buildInitramfs transforms `initramfs.xz` with system extensions.
func (i *Imager) buildInitramfs(ctx context.Context, report *reporter.Reporter) error {
	if len(i.prof.Input.SystemExtensions) == 0 {
		report.Report(reporter.Update{
			Message: "skipped initramfs rebuild (no system extensions)",
			Status:  reporter.StatusSkip,
		})

		// no system extensions, happy path
		i.initramfsPath = i.prof.Input.Initramfs.Path

		return nil
	}

	if i.prof.Output.Kind == profile.OutKindCmdline || i.prof.Output.Kind == profile.OutKindKernel {
		// these outputs don't use initramfs image
		return nil
	}

	printf := progressPrintf(report, reporter.Update{Message: "rebuilding initramfs with system extensions...", Status: reporter.StatusRunning})

	// copy the initramfs to a temporary location, as it's going to be modified during the extension build process
	tempInitramfsPath := filepath.Join(i.tempDir, "initramfs.xz")

	if err := utils.CopyFiles(printf, utils.SourceDestination(i.prof.Input.Initramfs.Path, tempInitramfsPath)); err != nil {
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

		if err := ext.Extract(ctx, extensionDir, i.prof.Arch, printf); err != nil {
			return err
		}
	}

	// rebuild initramfs
	builder := extensions.Builder{
		InitramfsPath:     i.initramfsPath,
		Arch:              i.prof.Arch,
		ExtensionTreePath: extensionsCheckoutDir,
		Printf:            printf,
	}

	if err := builder.Build(); err != nil {
		return err
	}

	report.Report(reporter.Update{
		Message: "initramfs ready",
		Status:  reporter.StatusSucceeded,
	})

	return nil
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
		cmdline.Append(
			constants.KernelParamEnvironment,
			constants.MetaValuesEnvVar+"="+i.prof.Customization.MetaContents.Encode(quirks.New(i.prof.Version).SupportsCompressedEncodedMETA()),
		)
	}

	// apply customization
	if err = cmdline.AppendAll(
		i.prof.Customization.ExtraKernelArgs,
		procfs.WithOverwriteArgs("console"),
		procfs.WithOverwriteArgs(constants.KernelParamPlatform),
		procfs.WithDeleteNegatedArgs(),
	); err != nil {
		return err
	}

	i.cmdline = cmdline.String()

	return nil
}

// buildUKI assembles the UKI and signs it.
func (i *Imager) buildUKI(ctx context.Context, report *reporter.Reporter) error {
	printf := progressPrintf(report, reporter.Update{Message: "building UKI...", Status: reporter.StatusRunning})

	i.sdBootPath = filepath.Join(i.tempDir, "systemd-boot.efi.signed")
	i.ukiPath = filepath.Join(i.tempDir, "vmlinuz.efi.signed")

	pcrSigner, err := i.prof.Input.SecureBoot.PCRSigner.GetSigner(ctx)
	if err != nil {
		return fmt.Errorf("failed to get PCR signer: %w", err)
	}

	securebootSigner, err := i.prof.Input.SecureBoot.SecureBootSigner.GetSigner(ctx)
	if err != nil {
		return fmt.Errorf("failed to get SecureBoot signer: %w", err)
	}

	builder := uki.Builder{
		Arch:       i.prof.Arch,
		Version:    i.prof.Version,
		SdStubPath: i.prof.Input.SDStub.Path,
		SdBootPath: i.prof.Input.SDBoot.Path,
		KernelPath: i.prof.Input.Kernel.Path,
		InitrdPath: i.initramfsPath,
		Cmdline:    i.cmdline,

		SecureBootSigner: securebootSigner,
		PCRSigner:        pcrSigner,

		OutSdBootPath: i.sdBootPath,
		OutUKIPath:    i.ukiPath,
	}

	if err := builder.Build(printf); err != nil {
		return err
	}

	report.Report(reporter.Update{
		Message: "UKI ready",
		Status:  reporter.StatusSucceeded,
	})

	return nil
}
