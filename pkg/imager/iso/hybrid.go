// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso

import "path/filepath"

// CreateHybrid creates an ISO image that supports both BIOS and UEFI booting.
func (options Options) CreateHybrid(printf func(string, ...any)) (Generator, error) {
	if _, err := options.CreateGRUB(printf); err != nil {
		return nil, err
	}

	if _, err := options.CreateUEFI(printf); err != nil {
		return nil, err
	}

	efiBootImg := filepath.Join(options.ScratchDir, "efiboot.img")

	return &ExecutorOptions{
		Command: "grub-mkrescue",
		Version: options.Version,
		Arguments: []string{
			"--compress=xz",
			"--output=" + options.OutPath,
			"--verbose",
			"--directory=/usr/lib/grub/i386-pc", // only for BIOS boot
			"-m", "efiboot.img",                 // exclude the EFI boot image from the ISO
			"-iso-level", "3",
			options.ScratchDir,
			"-eltorito-alt-boot",
			"-e", "--interval:appended_partition_2:all::", // use appended partition 2 for EFI
			"-append_partition", "2", "0xef", efiBootImg,
			"-appended_part_as_gpt",
			"-partition_cyl_align", // pad partition to cylinder boundary
			"all",
			"-partition_offset", "16", // support booting from USB
			"-iso_mbr_part_type", "0x83", // just to have more clear info when doing a fdisk -l
			"-no-emul-boot",
			"--",
		},
	}, nil
}
