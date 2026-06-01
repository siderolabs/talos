// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// PatchBlocklistsForDiskImage patches the GRUB boot.img and core.img with blocklist information
// for GPT+BIOS boot. This should be called after the disk partition layout is finalized.
//
// References (GRUB source tree inside orb VM):
// - grub-core/boot/i386/pc/boot.S: defines GRUB_BOOT_MACHINE_KERNEL_SECTOR and MBR code
// - include/grub/i386/pc/boot.h: GRUB_BOOT_MACHINE_KERNEL_SECTOR == 0x5c
// - util/setup.c: write_rootdev() patches boot.img fields and writes sector in LE64
// - core image embedded blocklist continuation at core.img offset 0x1F4.
func PatchBlocklistsForDiskImage(sectorSize uint, biosBootStartSector uint64, mountPrefix string) error {
	if sectorSize == 0 {
		return fmt.Errorf("sector size must be set to patch GRUB blocklists")
	}

	// Talos partition layout (GPT): EFI (gpt1, efiPartitionSizeBytes), BIOS (gpt2, 1MiB), BOOT (gpt3)
	// BIOS boot partition starts immediately after the EFI partition.
	const (
		bootImgKernelSectorOffset = 0x5c  // include/grub/i386/pc/boot.h (GRUB_BOOT_MACHINE_KERNEL_SECTOR)
		bootImgJumpOffset         = 0x66  // patched to NOP NOP (0x90 0x90) by grub-install on GPT
		coreImgBlocklistOffset    = 0x1f4 // embedded blocklist continuation inside core.img
	)

	bootImgPath := filepath.Join(mountPrefix, "boot.img")
	coreImgPath := filepath.Join(mountPrefix, "core.img")

	bootImg, err := os.ReadFile(bootImgPath)
	if err != nil {
		return fmt.Errorf("failed to read boot.img: %w", err)
	}

	// validate bootImgKernelSectorOffset and bootImgJumpOffset can be patched into bootImg
	if len(bootImg) < bootImgKernelSectorOffset+8 {
		return fmt.Errorf("boot.img is too small (%d bytes) to patch kernel sector offset at 0x%x", len(bootImg), bootImgKernelSectorOffset)
	}

	if len(bootImg) < bootImgJumpOffset+2 {
		return fmt.Errorf("boot.img is too small (%d bytes) to patch jump offset at 0x%x", len(bootImg), bootImgJumpOffset)
	}

	// Patch 1: tell boot.img where to find core.img (LE64 sector number at 0x5C)
	binary.LittleEndian.PutUint64(bootImg[bootImgKernelSectorOffset:], biosBootStartSector)

	// Patch 2: NOP the short jump at 0x66 for GPT installs (matches grub-install behavior)
	bootImg[bootImgJumpOffset] = 0x90
	bootImg[bootImgJumpOffset+1] = 0x90

	if err := os.WriteFile(bootImgPath, bootImg, 0o644); err != nil {
		return fmt.Errorf("failed to write patched boot.img: %w", err)
	}

	coreImg, err := os.ReadFile(coreImgPath)
	if err != nil {
		return fmt.Errorf("failed to read core.img: %w", err)
	}

	// validate coreImgBlocklistOffset can be patched into coreImg
	if len(coreImg) < coreImgBlocklistOffset+8 {
		return fmt.Errorf("core.img is too small (%d bytes) to patch blocklist offset at 0x%x", len(coreImg), coreImgBlocklistOffset)
	}

	// Patch 3: core.img embedded blocklist continuation (LE64) points to start+1
	//
	// The boot.img only loads the first sector of core.img, so the embedded blocklist
	// continuation must point to the second sector of core.img.
	binary.LittleEndian.PutUint64(coreImg[coreImgBlocklistOffset:], biosBootStartSector+1)

	if err := os.WriteFile(coreImgPath, coreImg, 0o644); err != nil {
		return fmt.Errorf("failed to write patched core.img: %w", err)
	}

	return nil
}
