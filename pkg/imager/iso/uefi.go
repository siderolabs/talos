// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/makefs"
)

// UEFIOptions describe the input for the CreateUEFI function.
type UEFIOptions struct {
	UKIPath    string
	SDBootPath string

	// A value in loader.conf secure-boot-enroll: off, manual, if-safe, force.
	SDBootSecureBootEnrollKeys string

	// optional, for auto-enrolling secureboot keys
	PlatformKeyPath    string
	KeyExchangeKeyPath string
	SignatureKeyPath   string

	Arch    string
	Version string

	ScratchDir string
	OutPath    string
}

const (
	// mib is the size of a megabyte.
	mib = 1024 * 1024
)

//go:embed loader.conf.tmpl
var loaderConfigTemplate string

// CreateUEFI creates an iso using a UKI, systemd-boot.
//
// The ISO created supports only booting in UEFI mode, and supports SecureBoot.
//
//nolint:gocyclo,cyclop
func CreateUEFI(printf func(string, ...any), options UEFIOptions) error {
	if err := os.MkdirAll(options.ScratchDir, 0o755); err != nil {
		return err
	}

	printf("preparing raw image")

	efiBootImg := filepath.Join(options.ScratchDir, "efiboot.img")

	// initial size
	isoSize := int64(10 * mib)

	for _, path := range []string{
		options.SDBootPath,
		options.UKIPath,
	} {
		st, err := os.Stat(path)
		if err != nil {
			return err
		}

		isoSize += (st.Size() + mib - 1) / mib * mib
	}

	if err := utils.CreateRawDisk(printf, efiBootImg, isoSize); err != nil {
		return err
	}

	printf("preparing loader.conf")

	var loaderConfigOut bytes.Buffer

	if err := template.Must(template.New("loader.conf").Parse(loaderConfigTemplate)).Execute(&loaderConfigOut, struct {
		SecureBootEnroll string
	}{
		SecureBootEnroll: options.SDBootSecureBootEnrollKeys,
	}); err != nil {
		return fmt.Errorf("error rendering loader.conf: %w", err)
	}

	printf("creating vFAT EFI image")

	fopts := []makefs.Option{
		makefs.WithLabel(constants.EFIPartitionLabel),
		makefs.WithReproducible(true),
	}

	if err := makefs.VFAT(efiBootImg, fopts...); err != nil {
		return err
	}

	if _, err := cmd.Run("mmd", "-i", efiBootImg, "::EFI"); err != nil {
		return err
	}

	if _, err := cmd.Run("mmd", "-i", efiBootImg, "::EFI/BOOT"); err != nil {
		return err
	}

	if _, err := cmd.Run("mmd", "-i", efiBootImg, "::EFI/Linux"); err != nil {
		return err
	}

	if _, err := cmd.Run("mmd", "-i", efiBootImg, "::loader"); err != nil {
		return err
	}

	if _, err := cmd.Run("mmd", "-i", efiBootImg, "::loader/keys"); err != nil {
		return err
	}

	if _, err := cmd.Run("mmd", "-i", efiBootImg, "::loader/keys/auto"); err != nil {
		return err
	}

	efiBootPath := "::EFI/BOOT/BOOTX64.EFI"

	if options.Arch == "arm64" {
		efiBootPath = "::EFI/BOOT/BOOTAA64.EFI"
	}

	if _, err := cmd.Run("mcopy", "-i", efiBootImg, options.SDBootPath, efiBootPath); err != nil {
		return err
	}

	if _, err := cmd.Run("mcopy", "-i", efiBootImg, options.UKIPath, fmt.Sprintf("::EFI/Linux/Talos-%s.efi", options.Version)); err != nil {
		return err
	}

	if _, err := cmd.RunContext(
		cmd.WithStdin(context.Background(), &loaderConfigOut),
		"mcopy", "-i", efiBootImg, "-", "::loader/loader.conf",
	); err != nil {
		return err
	}

	if options.PlatformKeyPath != "" {
		if _, err := cmd.Run("mcopy", "-i", efiBootImg, options.PlatformKeyPath, filepath.Join("::loader/keys/auto", constants.PlatformKeyAsset)); err != nil {
			return err
		}
	}

	if options.KeyExchangeKeyPath != "" {
		if _, err := cmd.Run("mcopy", "-i", efiBootImg, options.KeyExchangeKeyPath, filepath.Join("::loader/keys/auto", constants.KeyExchangeKeyAsset)); err != nil {
			return err
		}
	}

	if options.SignatureKeyPath != "" {
		if _, err := cmd.Run("mcopy", "-i", efiBootImg, options.SignatureKeyPath, filepath.Join("::loader/keys/auto", constants.SignatureKeyAsset)); err != nil {
			return err
		}
	}

	// fixup directory timestamps recursively
	if err := utils.TouchFiles(printf, options.ScratchDir); err != nil {
		return err
	}

	printf("creating ISO image")

	if _, err := cmd.Run(
		"xorriso",
		"-as",
		"mkisofs",
		"-V",
		"Talos Secure Boot ISO",
		"-e",
		"efiboot.img",
		"-no-emul-boot",
		"-o",
		options.OutPath,
		options.ScratchDir,
	); err != nil {
		return err
	}

	return nil
}
