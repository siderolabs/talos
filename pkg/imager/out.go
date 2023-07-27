// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/pkg/imager/filemap"
	"github.com/siderolabs/talos/pkg/imager/iso"
	"github.com/siderolabs/talos/pkg/imager/ova"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/imager/qemuimg"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func (i *Imager) outInitramfs(path string) error {
	return utils.CopyFiles(utils.SourceDestination(i.initramfsPath, path))
}

func (i *Imager) outKernel(path string) error {
	return utils.CopyFiles(utils.SourceDestination(i.prof.Input.Kernel.Path, path))
}

func (i *Imager) outUKI(path string) error {
	return utils.CopyFiles(utils.SourceDestination(i.ukiPath, path))
}

func (i *Imager) outISO(path string) error {
	scratchSpace := filepath.Join(i.tempDir, "iso")

	if i.prof.SecureBootEnabled() {
		return iso.CreateUEFI(iso.UEFIOptions{
			UKIPath:    i.ukiPath,
			SDBootPath: i.sdBootPath,

			PlatformKeyPath:    i.prof.Input.SecureBoot.PlatformKeyPath,
			KeyExchangeKeyPath: i.prof.Input.SecureBoot.KeyExchangeKeyPath,
			SignatureKeyPath:   i.prof.Input.SecureBoot.SignatureKeyPath,

			Arch:    i.prof.Arch,
			Version: i.prof.Version,

			ScratchDir: scratchSpace,
			OutPath:    path,
		})
	}

	return iso.CreateGRUB(iso.GRUBOptions{
		KernelPath:    i.prof.Input.Kernel.Path,
		InitramfsPath: i.initramfsPath,
		Cmdline:       i.cmdline,

		ScratchDir: scratchSpace,
		OutPath:    path,
	})
}

func (i *Imager) outImage(ctx context.Context, path string) error {
	if err := i.buildImage(ctx, path); err != nil {
		return err
	}

	switch i.prof.Output.ImageOptions.DiskFormat {
	case profile.DiskFormatRaw:
		return nil
	case profile.DiskFormatQCOW2:
		return qemuimg.Convert("raw", "qcow2", i.prof.Output.ImageOptions.DiskFormatOptions, path)
	case profile.DiskFormatVPC:
		return qemuimg.Convert("raw", "vpc", i.prof.Output.ImageOptions.DiskFormatOptions, path)
	case profile.DiskFormatOVA:
		scratchPath := filepath.Join(i.tempDir, "ova")

		return ova.CreateOVAFromRAW(fmt.Sprintf("%s-%s", i.prof.Platform, i.prof.Arch), path, i.prof.Arch, scratchPath, i.prof.Output.ImageOptions.DiskSize)
	case profile.DiskFormatUnknown:
		fallthrough
	default:
		return fmt.Errorf("unsupported disk format: %s", i.prof.Output.ImageOptions.DiskFormat)
	}
}

func (i *Imager) buildImage(ctx context.Context, path string) error {
	if err := utils.CreateRawDisk(path, i.prof.Output.ImageOptions.DiskSize); err != nil {
		return err
	}

	log.Print("attaching loopback device")

	var (
		loDevice string
		err      error
	)

	if loDevice, err = utils.Loattach(path); err != nil {
		return err
	}

	defer func() {
		log.Println("detaching loopback device")

		if e := utils.Lodetach(loDevice); e != nil {
			log.Println(e)
		}
	}()

	cmdline := procfs.NewCmdline(i.cmdline)

	opts := &install.Options{
		Disk:       loDevice,
		Platform:   i.prof.Platform,
		Arch:       i.prof.Arch,
		Board:      i.prof.Board,
		MetaValues: install.FromMeta(i.prof.Customization.MetaContents),

		ImageSecureboot: i.prof.SecureBootEnabled(),
		Version:         i.prof.Version,
		BootAssets: options.BootAssets{
			KernelPath:    i.prof.Input.Kernel.Path,
			InitramfsPath: i.initramfsPath,
			UKIPath:       i.ukiPath,
			SDBootPath:    i.sdBootPath,
		},
	}

	if opts.Board == "" {
		opts.Board = constants.BoardNone
	}

	installer, err := install.NewInstaller(ctx, cmdline, install.ModeImage, opts)
	if err != nil {
		return err
	}

	return installer.Install(ctx, install.ModeImage)
}

//nolint:gocyclo
func (i *Imager) outInstaller(ctx context.Context, path string) error {
	baseInstallerImg, err := i.prof.Input.BaseInstaller.Pull(ctx, i.prof.Arch)
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

	newInstallerImg, err = mutate.AppendLayers(newInstallerImg, baseLayers[0])
	if err != nil {
		return fmt.Errorf("failed to append layers: %w", err)
	}

	var artifacts []filemap.File

	if i.prof.SecureBootEnabled() {
		artifacts = append(artifacts,
			filemap.File{
				ImagePath:  fmt.Sprintf(constants.UKIAssetPath, i.prof.Arch),
				SourcePath: i.ukiPath,
			},
			filemap.File{
				ImagePath:  fmt.Sprintf(constants.SDBootAssetPath, i.prof.Arch),
				SourcePath: i.sdBootPath,
			},
		)
	} else {
		artifacts = append(artifacts,
			filemap.File{
				ImagePath:  fmt.Sprintf(constants.KernelAssetPath, i.prof.Arch),
				SourcePath: i.prof.Input.Kernel.Path,
			},
			filemap.File{
				ImagePath:  fmt.Sprintf(constants.InitramfsAssetPath, i.prof.Arch),
				SourcePath: i.initramfsPath,
			},
		)
	}

	artifactsLayer, err := filemap.Layer(artifacts)
	if err != nil {
		return fmt.Errorf("failed to create artifacts layer: %w", err)
	}

	newInstallerImg, err = mutate.AppendLayers(newInstallerImg, artifactsLayer)
	if err != nil {
		return fmt.Errorf("failed to append artifacts layer: %w", err)
	}

	ref, err := name.ParseReference(i.prof.Input.BaseInstaller.ImageRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %w", err)
	}

	return tarball.WriteToFile(path, ref, newInstallerImg)
}
