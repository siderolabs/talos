// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso

import (
	"bytes"
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

// UEFIOptions describe the input for the CreateUEFI function.
type UEFIOptions struct {
	UKIPath    string
	SDBootPath string

	// A value in loader.conf secure-boot-enroll: off, manual, if-safe, force.
	SDBootSecureBootEnrollKeys string

	// UKISigningCertDer is the DER encoded UKI signing certificate.
	UKISigningCertDerPath string

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
func (options Options) CreateUEFI(printf func(string, ...any)) (Generator, error) {
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

	if err := utils.CreateRawDisk(printf, efiBootImg, isoSize); err != nil {
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
	}

	if err := makefs.VFAT(efiBootImg, fopts...); err != nil {
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

	if _, err := cmd.Run(
		"mcopy",
		"-s", // recursive
		"-p", // preserve attributes
		"-Q", // quit on error
		"-m", // preserve modification time
		"-i",
		efiBootImg,
		filepath.Join(options.ScratchDir, "EFI"),
		filepath.Join(options.ScratchDir, "loader"),
		"::",
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
