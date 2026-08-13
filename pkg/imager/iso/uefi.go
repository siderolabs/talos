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
	"github.com/siderolabs/go-copy/copy"

	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/makefs"
)

const (
	// mib is the size of a megabyte.
	mib = 1024 * 1024
)

//go:embed loader.conf.tmpl
var loaderConfigTemplate string

// fatSectorsPerCluster picks the cluster size for the ESP holding the UKI.
//
// mkfs.vfat defaults to a single 512 byte sector per cluster at the sizes we
// build here, which leaves the ESP with a ~900 KiB FAT per copy and a cluster
// chain of ~200k entries for a single ~100 MiB UKI. Firmware walks that chain
// on every boot, which is painful on slow media (e.g. an optical drive).
//
// The cluster size is capped so that the cluster count stays above the FAT32
// minimum of 65525: most implementations (EDK2 included) derive the FAT type
// from the cluster count, so a FAT32 filesystem below that floor reads back as
// FAT16 and fails to mount.
func fatSectorsPerCluster(sizeBytes int64) uint {
	const (
		sectorSize = 512
		// FAT32 minimum cluster count, plus room for the reserved sectors and the FATs themselves.
		minClusters = 65525 + 2048
		maxSPC      = 8
	)

	sectors := sizeBytes / sectorSize

	spc := uint(1)

	for next := spc * 2; next <= maxSPC; next *= 2 {
		if sectors/int64(next) < minClusters {
			break
		}

		spc = next
	}

	return spc
}

// CreateUEFI creates an iso using a UKI, systemd-boot.
//
// The ISO created supports only booting in UEFI mode, and supports SecureBoot.
//
//nolint:gocyclo,cyclop
func (options Options) CreateUEFI(ctx context.Context, printf func(string, ...any)) (Generator, error) {
	if err := os.MkdirAll(options.ScratchDir, 0o755); err != nil {
		return nil, err
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
			return nil, err
		}

		isoSize += (st.Size() + mib - 1) / mib * mib
	}

	if err := utils.CreateRawDisk(printf, efiBootImg, isoSize, true); err != nil {
		return nil, err
	}

	printf("preparing loader.conf")

	var loaderConfigOut bytes.Buffer

	if err := template.Must(template.New("loader.conf").Parse(loaderConfigTemplate)).Execute(&loaderConfigOut, struct {
		SecureBootEnroll string
	}{
		SecureBootEnroll: options.SDBootSecureBootEnrollKeys,
	}); err != nil {
		return nil, fmt.Errorf("error rendering loader.conf: %w", err)
	}

	printf("creating vFAT EFI image")

	fopts := []makefs.Option{
		makefs.WithLabel(constants.EFIPartitionLabel),
		makefs.WithReproducible(true),
		makefs.WithSectorsPerCluster(fatSectorsPerCluster(isoSize)),
	}

	if err := makefs.VFAT(ctx, efiBootImg, fopts...); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Join(options.ScratchDir, "EFI/Linux"), 0o755); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Join(options.ScratchDir, "EFI/BOOT"), 0o755); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Join(options.ScratchDir, "loader"), 0o755); err != nil {
		return nil, err
	}

	efiBootPath := "EFI/BOOT/BOOTX64.EFI"

	if options.Arch == "arm64" {
		efiBootPath = "EFI/BOOT/BOOTAA64.EFI"
	}

	if err := copy.File(options.SDBootPath, filepath.Join(options.ScratchDir, efiBootPath)); err != nil {
		return nil, err
	}

	if err := copy.File(options.UKIPath, filepath.Join(options.ScratchDir, fmt.Sprintf("EFI/Linux/Talos-%s.efi", options.Version))); err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(options.ScratchDir, "loader/loader.conf"), loaderConfigOut.Bytes(), 0o644); err != nil {
		return nil, err
	}

	// Ship a placeholder random seed, marked read-only below.
	//
	// systemd-boot v261 grew a code path which creates /loader/random-seed when the file is missing and the
	// firmware handed out entropy, and it does so without checking whether the medium can be written to. On
	// an ISO booted as an El Torito CD/DVD that means a create is attempted against read-only media, and some
	// firmware faults there rather than returning an error, taking the machine down with a synchronous
	// exception right after a boot entry is selected: https://github.com/siderolabs/talos/issues/14029
	//
	// Providing the file keeps systemd-boot out of that path entirely: the open fails with a plain error and
	// the seed is left alone, which is what systemd <= 260 did. The contents are never consumed (the file is
	// read-only, and so is the medium), so a fixed placeholder keeps the ISO reproducible.
	if err := os.WriteFile(filepath.Join(options.ScratchDir, "loader/random-seed"), make([]byte, 32), 0o444); err != nil {
		return nil, err
	}

	if options.UKISigningCertDerPath != "" {
		if err := os.MkdirAll(filepath.Join(options.ScratchDir, "EFI/keys"), 0o755); err != nil {
			return nil, err
		}

		if err := copy.File(options.UKISigningCertDerPath, filepath.Join(options.ScratchDir, "EFI/keys/uki-signing-cert.der")); err != nil {
			return nil, err
		}
	}

	if options.PlatformKeyPath != "" || options.KeyExchangeKeyPath != "" || options.SignatureKeyPath != "" {
		if err := os.MkdirAll(filepath.Join(options.ScratchDir, "loader/keys/auto"), 0o755); err != nil {
			return nil, err
		}
	}

	if options.PlatformKeyPath != "" {
		if err := copy.File(options.PlatformKeyPath, filepath.Join(options.ScratchDir, "loader/keys/auto", constants.PlatformKeyAsset)); err != nil {
			return nil, err
		}
	}

	if options.KeyExchangeKeyPath != "" {
		if err := copy.File(options.KeyExchangeKeyPath, filepath.Join(options.ScratchDir, "loader/keys/auto", constants.KeyExchangeKeyAsset)); err != nil {
			return nil, err
		}
	}

	if options.SignatureKeyPath != "" {
		if err := copy.File(options.SignatureKeyPath, filepath.Join(options.ScratchDir, "loader/keys/auto", constants.SignatureKeyAsset)); err != nil {
			return nil, err
		}
	}

	// fixup directory timestamps recursively
	if err := utils.TouchFiles(printf, options.ScratchDir); err != nil {
		return nil, err
	}

	if _, err := cmd.RunWithOptions(
		ctx,
		"mcopy",
		[]string{
			"-s", // recursive
			"-p", // preserve attributes
			"-Q", // quit on error
			"-m", // preserve modification time
			"-i",
			efiBootImg,
			filepath.Join(options.ScratchDir, "EFI"),
			filepath.Join(options.ScratchDir, "loader"),
			"::",
		},
	); err != nil {
		return nil, err
	}

	// Mark the random seed read-only, so that systemd-boot >= 261.2 recognizes it as a seed it should not
	// manage, and so that the open in older versions fails before any write is attempted.
	if _, err := cmd.RunWithOptions(
		ctx,
		"mattrib",
		[]string{
			"-i",
			efiBootImg,
			"+r",
			"::/loader/random-seed",
		},
	); err != nil {
		return nil, err
	}

	printf("creating ISO image")

	return &ExecutorOptions{
		Command: "xorrisofs",
		Version: options.Version,
		Arguments: []string{
			"-e", "--interval:appended_partition_2:all::", // use appended partition 2 for EFI
			"-append_partition", "2", "0xef", efiBootImg,
			"-partition_cyl_align", // pad partition to cylinder boundary
			"all",
			"-partition_offset", "16", // support booting from USB
			"-iso_mbr_part_type", "0x83", // just to have more clear info when doing a fdisk -l
			"-no-emul-boot",
			"-m", "efiboot.img", // exclude the EFI boot image from the ISO
			"-iso-level", "3",
			"-o", options.OutPath,
			options.ScratchDir,
			"--",
		},
	}, nil
}
