// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/makefs"
	"github.com/siderolabs/talos/pkg/version"
)

const (
	// MiB is the size of a megabyte.
	MiB = 1024 * 1024
	// UKIISOSizeAMD64 is the size of the AMD64 UKI ISO.
	UKIISOSizeAMD64 = 80 * MiB
	// UKIISOSizeARM64 is the size of the ARM64 UKI ISO.
	UKIISOSizeARM64 = 120 * MiB
)

// CreateISO creates an iso by invoking the `grub-mkrescue` command.
func CreateISO(iso, dir string) error {
	args := []string{
		"--compress=xz",
		"--output=" + iso,
		dir,
	}

	if epoch, ok, err := SourceDateEpoch(); err != nil {
		return err
	} else if ok {
		// set EFI FAT image serial number
		if err := os.Setenv("GRUB_FAT_SERIAL_NUMBER", fmt.Sprintf("%x", uint32(epoch))); err != nil {
			return err
		}

		args = append(args,
			"--",
			"-volume_date", "all_file_dates", fmt.Sprintf("=%d", epoch),
			"-volume_date", "uuid", time.Unix(epoch, 0).Format("2006010215040500"),
		)
	}

	_, err := cmd.Run("grub-mkrescue", args...)
	if err != nil {
		return fmt.Errorf("failed to create ISO: %w", err)
	}

	return nil
}

// CreateUKIISO creates an iso using a UKI and UEFI only.
// nolint:gocyclo
func CreateUKIISO(iso, dir, arch string) error {
	isoDir := filepath.Join(dir, "iso")

	if err := os.MkdirAll(isoDir, 0o755); err != nil {
		return err
	}

	defer os.RemoveAll(isoDir) // nolint:errcheck

	efiBootImg := filepath.Join(isoDir, "efiboot.img")

	f, err := os.Create(efiBootImg)
	if err != nil {
		return err
	}

	isoSize := UKIISOSizeAMD64

	if arch == "arm64" {
		isoSize = UKIISOSizeARM64
	}

	if err := f.Truncate(int64(isoSize)); err != nil {
		return err
	}

	defer f.Close() // nolint:errcheck

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

	if arch == "arm64" {
		efiBootPath = "::EFI/BOOT/BOOTAA64.EFI"
	}

	if _, err := cmd.Run("mcopy", "-i", efiBootImg, filepath.Join(dir, "systemd-boot.efi.signed"), efiBootPath); err != nil {
		return err
	}

	if _, err := cmd.Run("mcopy", "-i", efiBootImg, filepath.Join(dir, "vmlinuz.efi.signed"), fmt.Sprintf("::EFI/Linux/Talos-%s.efi", version.Tag)); err != nil {
		return err
	}

	if _, err := cmd.Run("mcopy", "-i", efiBootImg, filepath.Join(dir, "PK.auth"), "::loader/keys/auto/PK.auth"); err != nil {
		return err
	}

	if _, err := cmd.Run("mcopy", "-i", efiBootImg, filepath.Join(dir, "PK.auth"), "::loader/keys/auto/KEK.auth"); err != nil {
		return err
	}

	if _, err := cmd.Run("mcopy", "-i", efiBootImg, filepath.Join(dir, "PK.auth"), "::loader/keys/auto/db.auth"); err != nil {
		return err
	}

	// fixup directory timestamps recursively
	if err := TouchFiles(dir); err != nil {
		return err
	}

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
		iso,
		isoDir,
	); err != nil {
		return err
	}

	return nil
}
