// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	randv2 "math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/freddierice/go-losetup/v2"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/secureboot/database"
	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
	"github.com/siderolabs/talos/pkg/imager/filemap"
	"github.com/siderolabs/talos/pkg/imager/iso"
	"github.com/siderolabs/talos/pkg/imager/ova"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/imager/qemuimg"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/reporter"
)

func (i *Imager) outInitramfs(path string, report *reporter.Reporter) error {
	printf := progressPrintf(report, reporter.Update{Message: "copying initramfs...", Status: reporter.StatusRunning})

	if err := utils.CopyFiles(printf, utils.SourceDestination(i.initramfsPath, path)); err != nil {
		return err
	}

	report.Report(reporter.Update{Message: "initramfs output ready", Status: reporter.StatusSucceeded})

	return nil
}

func (i *Imager) outKernel(path string, report *reporter.Reporter) error {
	printf := progressPrintf(report, reporter.Update{Message: "copying kernel...", Status: reporter.StatusRunning})

	if err := utils.CopyFiles(printf, utils.SourceDestination(i.prof.Input.Kernel.Path, path)); err != nil {
		return err
	}

	report.Report(reporter.Update{Message: "kernel output ready", Status: reporter.StatusSucceeded})

	return nil
}

func (i *Imager) outUKI(path string, report *reporter.Reporter) error {
	printf := progressPrintf(report, reporter.Update{Message: "copying kernel...", Status: reporter.StatusRunning})

	if err := utils.CopyFiles(printf, utils.SourceDestination(i.ukiPath, path)); err != nil {
		return err
	}

	report.Report(reporter.Update{Message: "UKI output ready", Status: reporter.StatusSucceeded})

	return nil
}

func (i *Imager) outCmdline(path string) error {
	return os.WriteFile(path, []byte(i.cmdline), 0o644)
}

//nolint:gocyclo
func (i *Imager) outISO(ctx context.Context, path string, report *reporter.Reporter) error {
	printf := progressPrintf(report, reporter.Update{Message: "building ISO...", Status: reporter.StatusRunning})

	scratchSpace := filepath.Join(i.tempDir, "iso")

	var (
		err                error
		zeroContainerAsset profile.ContainerAsset
	)

	if i.prof.Input.ImageCache != zeroContainerAsset {
		if err := os.MkdirAll(filepath.Join(scratchSpace, "imagecache"), 0o755); err != nil {
			return err
		}

		if err := i.prof.Input.ImageCache.Extract(ctx, filepath.Join(scratchSpace, "imagecache"), i.prof.Arch, printf); err != nil {
			return err
		}
	}

	if i.prof.SecureBootEnabled() {
		isoOptions := pointer.SafeDeref(i.prof.Output.ISOOptions)

		var signer pesign.CertificateSigner

		signer, err = i.prof.Input.SecureBoot.SecureBootSigner.GetSigner(ctx)
		if err != nil {
			return fmt.Errorf("failed to get SecureBoot signer: %w", err)
		}

		derCrtPath := filepath.Join(i.tempDir, "uki.der")

		if err = os.WriteFile(derCrtPath, signer.Certificate().Raw, 0o600); err != nil {
			return fmt.Errorf("failed to write uki.der: %w", err)
		}

		options := iso.UEFIOptions{
			UKIPath:    i.ukiPath,
			SDBootPath: i.sdBootPath,

			SDBootSecureBootEnrollKeys: isoOptions.SDBootEnrollKeys.String(),

			UKISigningCertDerPath: derCrtPath,

			PlatformKeyPath:    i.prof.Input.SecureBoot.PlatformKeyPath,
			KeyExchangeKeyPath: i.prof.Input.SecureBoot.KeyExchangeKeyPath,
			SignatureKeyPath:   i.prof.Input.SecureBoot.SignatureKeyPath,

			Arch:    i.prof.Arch,
			Version: i.prof.Version,

			ScratchDir: scratchSpace,
			OutPath:    path,
		}

		if i.prof.Input.SecureBoot.PlatformKeyPath == "" {
			report.Report(reporter.Update{Message: "generating SecureBoot database...", Status: reporter.StatusRunning})

			// generate the database automatically from provided values
			enrolledPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: signer.Certificate().Raw,
			})

			var entries []database.Entry

			entries, err = database.Generate(enrolledPEM, signer, database.IncludeWellKnownCertificates(i.prof.Input.SecureBoot.IncludeWellKnownCerts))
			if err != nil {
				return fmt.Errorf("failed to generate database: %w", err)
			}

			for _, entry := range entries {
				entryPath := filepath.Join(i.tempDir, entry.Name)

				if err = os.WriteFile(entryPath, entry.Contents, 0o600); err != nil {
					return err
				}

				switch entry.Name {
				case constants.PlatformKeyAsset:
					options.PlatformKeyPath = entryPath
				case constants.KeyExchangeKeyAsset:
					options.KeyExchangeKeyPath = entryPath
				case constants.SignatureKeyAsset:
					options.SignatureKeyPath = entryPath
				default:
					return fmt.Errorf("unknown database entry: %s", entry.Name)
				}
			}
		} else {
			options.PlatformKeyPath = i.prof.Input.SecureBoot.PlatformKeyPath
			options.KeyExchangeKeyPath = i.prof.Input.SecureBoot.KeyExchangeKeyPath
			options.SignatureKeyPath = i.prof.Input.SecureBoot.SignatureKeyPath
		}

		err = iso.CreateUEFI(printf, options)
	} else {
		err = iso.CreateGRUB(printf, iso.GRUBOptions{
			KernelPath:    i.prof.Input.Kernel.Path,
			InitramfsPath: i.initramfsPath,
			Cmdline:       i.cmdline,
			Version:       i.prof.Version,

			ScratchDir: scratchSpace,
			OutPath:    path,
		})
	}

	if err != nil {
		return err
	}

	report.Report(reporter.Update{Message: "ISO ready", Status: reporter.StatusSucceeded})

	return nil
}

func (i *Imager) outImage(ctx context.Context, path string, report *reporter.Reporter) error {
	printf := progressPrintf(report, reporter.Update{Message: "creating disk image...", Status: reporter.StatusRunning})

	if err := i.buildImage(ctx, path, printf); err != nil {
		return err
	}

	switch i.prof.Output.ImageOptions.DiskFormat {
	case profile.DiskFormatRaw:
		// nothing to do
	case profile.DiskFormatQCOW2:
		if err := qemuimg.Convert("raw", "qcow2", i.prof.Output.ImageOptions.DiskFormatOptions, path, printf); err != nil {
			return err
		}
	case profile.DiskFormatVPC:
		if err := qemuimg.Convert("raw", "vpc", i.prof.Output.ImageOptions.DiskFormatOptions, path, printf); err != nil {
			return err
		}
	case profile.DiskFormatOVA:
		scratchPath := filepath.Join(i.tempDir, "ova")

		if err := ova.CreateOVAFromRAW(path, i.prof.Arch, scratchPath, i.prof.Output.ImageOptions.DiskSize, printf); err != nil {
			return err
		}
	case profile.DiskFormatUnknown:
		fallthrough
	default:
		return fmt.Errorf("unsupported disk format: %s", i.prof.Output.ImageOptions.DiskFormat)
	}

	report.Report(reporter.Update{Message: "disk image ready", Status: reporter.StatusSucceeded})

	return nil
}

//nolint:gocyclo
func (i *Imager) buildImage(ctx context.Context, path string, printf func(string, ...any)) error {
	if err := utils.CreateRawDisk(printf, path, i.prof.Output.ImageOptions.DiskSize); err != nil {
		return err
	}

	printf("attaching loopback device")

	var (
		loDevice           losetup.Device
		err                error
		zeroContainerAsset profile.ContainerAsset
	)

	for range 10 {
		loDevice, err = losetup.Attach(path, 0, false)
		if err != nil {
			if errors.Is(err, unix.EBUSY) {
				spraySleep := max(randv2.ExpFloat64(), 2.0)

				printf("retrying after %v seconds", spraySleep)

				time.Sleep(time.Duration(spraySleep * float64(time.Second)))

				continue
			}

			return fmt.Errorf("failed to attach loopback device: %w", err)
		}

		printf("attached loopback device: %s", loDevice.Path())

		break
	}

	defer func() {
		printf("detaching loopback device %s", loDevice.Path())

		if e := loDevice.Detach(); e != nil {
			log.Println(e)
		}
	}()

	cmdline := procfs.NewCmdline(i.cmdline)

	scratchSpace := filepath.Join(i.tempDir, "image")

	opts := &install.Options{
		Disk:       loDevice.Path(),
		Platform:   i.prof.Platform,
		Arch:       i.prof.Arch,
		Board:      i.prof.Board,
		MetaValues: install.FromMeta(i.prof.Customization.MetaContents),

		ImageSecureboot: i.prof.SecureBootEnabled(),
		Version:         i.prof.Version,
		BootAssets: options.BootAssets{
			KernelPath:      i.prof.Input.Kernel.Path,
			InitramfsPath:   i.initramfsPath,
			UKIPath:         i.ukiPath,
			SDBootPath:      i.sdBootPath,
			DTBPath:         i.prof.Input.DTB.Path,
			UBootPath:       i.prof.Input.UBoot.Path,
			RPiFirmwarePath: i.prof.Input.RPiFirmware.Path,
		},
		MountPrefix: scratchSpace,
		Printf:      printf,
	}

	if i.overlayInstaller != nil {
		opts.OverlayInstaller = i.overlayInstaller
		opts.ExtraOptions = i.prof.Overlay.ExtraOptions
		opts.OverlayExtractedDir = i.tempDir
	}

	if opts.Board == "" {
		opts.Board = constants.BoardNone
	}

	if i.prof.Input.ImageCache != zeroContainerAsset {
		imageCacheDir := filepath.Join(i.tempDir, "imagecache")

		if err := os.MkdirAll(imageCacheDir, 0o755); err != nil {
			return err
		}

		if err := i.prof.Input.ImageCache.Extract(ctx, imageCacheDir, i.prof.Arch, printf); err != nil {
			return err
		}

		opts.ImageCachePath = imageCacheDir
	}

	installer, err := install.NewInstaller(ctx, cmdline, install.ModeImage, opts)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	if err := installer.Install(ctx, install.ModeImage); err != nil {
		return fmt.Errorf("failed to install: %w", err)
	}

	return nil
}

//nolint:gocyclo,cyclop
func (i *Imager) outInstaller(ctx context.Context, path string, report *reporter.Reporter) error {
	printf := progressPrintf(report, reporter.Update{Message: "building installer...", Status: reporter.StatusRunning})

	baseInstallerImg, err := i.prof.Input.BaseInstaller.Pull(ctx, i.prof.Arch, printf)
	if err != nil {
		return err
	}

	baseLayers, err := baseInstallerImg.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	configFile, err := baseInstallerImg.ConfigFile()
	if err != nil {
		return fmt.Errorf("failed to get config file: %w", err)
	}

	config := *configFile.Config.DeepCopy()

	printf("creating empty image")

	newInstallerImg := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	newInstallerImg = mutate.ConfigMediaType(newInstallerImg, types.OCIConfigJSON)

	newInstallerImg, err = mutate.Config(newInstallerImg, config)
	if err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	newInstallerImg, err = mutate.CreatedAt(newInstallerImg, v1.Time{Time: time.Now()})
	if err != nil {
		return fmt.Errorf("failed to set created at: %w", err)
	}

	// Talos v1.5+ optimizes the install layers to be easily replaceable with new artifacts
	// other Talos versions will have an overhead of artifacts being stored twice
	if len(baseLayers) == 2 {
		// optimized for installer image for artifacts replacements
		baseLayers = baseLayers[:1]
	}

	newInstallerImg, err = mutate.AppendLayers(newInstallerImg, baseLayers...)
	if err != nil {
		return fmt.Errorf("failed to append layers: %w", err)
	}

	var artifacts []filemap.File

	printf("generating artifacts layer")

	if i.prof.SecureBootEnabled() {
		artifacts = append(artifacts,
			filemap.File{
				ImagePath:  strings.TrimLeft(fmt.Sprintf(constants.UKIAssetPath, i.prof.Arch), "/"),
				SourcePath: i.ukiPath,
			},
			filemap.File{
				ImagePath:  strings.TrimLeft(fmt.Sprintf(constants.SDBootAssetPath, i.prof.Arch), "/"),
				SourcePath: i.sdBootPath,
			},
		)
	} else {
		artifacts = append(artifacts,
			filemap.File{
				ImagePath:  strings.TrimLeft(fmt.Sprintf(constants.KernelAssetPath, i.prof.Arch), "/"),
				SourcePath: i.prof.Input.Kernel.Path,
			},
			filemap.File{
				ImagePath:  strings.TrimLeft(fmt.Sprintf(constants.InitramfsAssetPath, i.prof.Arch), "/"),
				SourcePath: i.initramfsPath,
			},
		)
	}

	if !quirks.New(i.prof.Version).SupportsOverlay() {
		for _, extraArtifact := range []struct {
			sourcePath string
			imagePath  string
		}{
			{
				sourcePath: i.prof.Input.DTB.Path,
				imagePath:  strings.TrimLeft(fmt.Sprintf(constants.DTBAssetPath, i.prof.Arch), "/"),
			},
			{
				sourcePath: i.prof.Input.UBoot.Path,
				imagePath:  strings.TrimLeft(fmt.Sprintf(constants.UBootAssetPath, i.prof.Arch), "/"),
			},
			{
				sourcePath: i.prof.Input.RPiFirmware.Path,
				imagePath:  strings.TrimLeft(fmt.Sprintf(constants.RPiFirmwareAssetPath, i.prof.Arch), "/"),
			},
		} {
			if extraArtifact.sourcePath == "" {
				continue
			}

			var extraFiles []filemap.File

			extraFiles, err = filemap.Walk(extraArtifact.sourcePath, extraArtifact.imagePath)
			if err != nil {
				return fmt.Errorf("failed to walk extra artifact %s: %w", extraArtifact.sourcePath, err)
			}

			artifacts = append(artifacts, extraFiles...)
		}
	}

	artifactsLayer, err := filemap.Layer(artifacts)
	if err != nil {
		return fmt.Errorf("failed to create artifacts layer: %w", err)
	}

	newInstallerImg, err = mutate.AppendLayers(newInstallerImg, artifactsLayer)
	if err != nil {
		return fmt.Errorf("failed to append artifacts layer: %w", err)
	}

	if i.overlayInstaller != nil {
		tempOverlayPath := filepath.Join(i.tempDir, "overlay-installer", constants.ImagerOverlayBasePath)

		if err := os.MkdirAll(tempOverlayPath, 0o755); err != nil {
			return fmt.Errorf("failed to create overlay directory: %w", err)
		}

		if err := i.prof.Input.OverlayInstaller.Extract(
			ctx,
			tempOverlayPath,
			i.prof.Arch,
			progressPrintf(report, reporter.Update{Message: "pulling overlay for installer...", Status: reporter.StatusRunning}),
		); err != nil {
			return err
		}

		extraOpts, internalErr := yaml.Marshal(i.prof.Overlay.ExtraOptions)
		if internalErr != nil {
			return fmt.Errorf("failed to marshal extra options: %w", internalErr)
		}

		if internalErr = os.WriteFile(filepath.Join(i.tempDir, constants.ImagerOverlayExtraOptionsPath), extraOpts, 0o644); internalErr != nil {
			return fmt.Errorf("failed to write extra options yaml: %w", internalErr)
		}

		printf("generating overlay installer layer")

		var overlayArtifacts []filemap.File

		for _, extraArtifact := range []struct {
			sourcePath string
			imagePath  string
		}{
			{
				sourcePath: filepath.Join(i.tempDir, "overlay-installer", constants.ImagerOverlayArtifactsPath),
				imagePath:  strings.TrimLeft(constants.ImagerOverlayArtifactsPath, "/"),
			},
			{
				sourcePath: filepath.Join(i.tempDir, "overlay-installer", constants.ImagerOverlayInstallersPath, i.prof.Overlay.Name),
				imagePath:  strings.TrimLeft(constants.ImagerOverlayInstallerDefaultPath, "/"),
			},
			{
				sourcePath: filepath.Join(i.tempDir, constants.ImagerOverlayExtraOptionsPath),
				imagePath:  strings.TrimLeft(constants.ImagerOverlayExtraOptionsPath, "/"),
			},
		} {
			var extraFiles []filemap.File

			extraFiles, err = filemap.Walk(extraArtifact.sourcePath, extraArtifact.imagePath)
			if err != nil {
				return fmt.Errorf("failed to walk extra artifact %s: %w", extraArtifact.sourcePath, err)
			}

			overlayArtifacts = append(overlayArtifacts, extraFiles...)
		}

		overlayArtifactsLayer, internalErr := filemap.Layer(overlayArtifacts)
		if internalErr != nil {
			return fmt.Errorf("failed to create overlay artifacts layer: %w", internalErr)
		}

		newInstallerImg, internalErr = mutate.AppendLayers(newInstallerImg, overlayArtifactsLayer)
		if internalErr != nil {
			return fmt.Errorf("failed to append overlay artifacts layer: %w", internalErr)
		}
	}

	ref, err := name.ParseReference(i.prof.Input.BaseInstaller.ImageRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %w", err)
	}

	printf("writing image tarball")

	if err := tarball.WriteToFile(path, ref, newInstallerImg); err != nil {
		return fmt.Errorf("failed to write image tarball: %w", err)
	}

	report.Report(reporter.Update{Message: "installer container image ready", Status: reporter.StatusSucceeded})

	return nil
}
