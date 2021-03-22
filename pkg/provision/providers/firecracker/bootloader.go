// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem/vfat"
	"github.com/talos-systems/go-blockdevice/blockdevice/partition/gpt"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/provision/internal/vmlinuz"
)

const diskImageSectorSize = 512

// BootLoader extracts kernel (vmlinux) and initrd (initramfs.xz) from Talos disk image.
type BootLoader struct {
	diskF *os.File

	bootPartitionReader *io.SectionReader

	bootFs *vfat.FileSystem

	kernelTempPath, initrdTempPath string
}

// BootAssets is what BootLoader extracts from the disk image.
type BootAssets struct {
	KernelPath string
	InitrdPath string
}

// NewBootLoader creates boot loader for the disk image.
func NewBootLoader(diskImage string) (*BootLoader, error) {
	b := &BootLoader{}

	var err error

	b.diskF, err = os.Open(diskImage)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// ExtractAssets from disk image.
func (b *BootLoader) ExtractAssets() (assets BootAssets, err error) {
	if err = b.findBootPartition(); err != nil {
		return assets, err
	}

	if err = b.openFilesystem(); err != nil {
		return assets, err
	}

	var label string

	if label, err = b.findLabel(); err != nil {
		return assets, err
	}

	if err := b.extractKernel(label); err != nil {
		return assets, err
	}

	if err := b.extractInitrd(label); err != nil {
		return assets, err
	}

	assets = BootAssets{
		KernelPath: b.kernelTempPath,
		InitrdPath: b.initrdTempPath,
	}

	return assets, nil
}

// Close the bootloader.
func (b *BootLoader) Close() error {
	if b.kernelTempPath != "" {
		os.Remove(b.kernelTempPath) //nolint:errcheck
		b.kernelTempPath = ""
	}

	if b.initrdTempPath != "" {
		os.Remove(b.initrdTempPath) //nolint:errcheck
		b.initrdTempPath = ""
	}

	if b.diskF != nil {
		if err := b.diskF.Close(); err != nil {
			return err
		}

		b.diskF = nil
	}

	return nil
}

func (b *BootLoader) findBootPartition() error {
	diskTable, err := gpt.Open(b.diskF)
	if err != nil {
		return fmt.Errorf("error creating GPT object: %w", err)
	}

	if err = diskTable.Read(); err != nil {
		return fmt.Errorf("error reading GPT: %w", err)
	}

	var bootPartition *gpt.Partition

	for _, part := range diskTable.Partitions().Items() {
		// TODO: should we do better matching here
		if part.Number == 1 {
			bootPartition = part

			break
		}
	}

	if bootPartition == nil {
		return fmt.Errorf("no boot partition found")
	}

	b.bootPartitionReader = io.NewSectionReader(b.diskF, int64(bootPartition.FirstLBA)*diskImageSectorSize, int64(bootPartition.Length())*diskImageSectorSize)

	return nil
}

func (b *BootLoader) openFilesystem() error {
	sb := &vfat.SuperBlock{}

	if _, err := b.bootPartitionReader.Seek(sb.Offset(), io.SeekStart); err != nil {
		return fmt.Errorf("error seeking boot partition: %w", err)
	}

	err := binary.Read(b.bootPartitionReader, binary.BigEndian, sb)
	if err != nil {
		return fmt.Errorf("error reading vfat superblock: %w", err)
	}

	if !sb.Is() {
		return fmt.Errorf("corrupt vfat superblock")
	}

	b.bootFs, err = vfat.NewFileSystem(b.bootPartitionReader, sb)
	if err != nil {
		return fmt.Errorf("error initializing FAT32 filesystem: %w", err)
	}

	return nil
}

func (b *BootLoader) findLabel() (label string, err error) {
	// Parse the syslinux.cfg first, for backwards compatibility.
	var cfg *vfat.File

	if cfg, err = b.bootFs.Open("/syslinux/syslinux.cfg"); err != nil {
		return label, fmt.Errorf("failed to open syslinux.cfg: %w", err)
	}

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(cfg); err != nil {
		return label, err
	}

	re := regexp.MustCompile(`^DEFAULT\s(.*)`)
	matches := re.FindSubmatch(buf.Bytes())

	if len(matches) != 2 {
		return label, fmt.Errorf("expected 2 matches, got %d", len(matches))
	}

	label = string(matches[1])

	return label, nil
}

func (b *BootLoader) extractKernel(label string) error {
	path := filepath.Join("/", label, constants.KernelAsset)

	r, err := b.bootFs.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open kernel asset %q: %w", path, err)
	}

	kernelR, err := vmlinuz.Decompress(bufio.NewReader(r))
	if err != nil {
		return fmt.Errorf("error decompressing kernel: %w", err)
	}

	defer kernelR.Close() //nolint:errcheck

	tempF, err := ioutil.TempFile("", "talos")
	if err != nil {
		return fmt.Errorf("error creating temporary kernel image file: %w", err)
	}

	defer tempF.Close() //nolint:errcheck

	if _, err = io.Copy(tempF, kernelR); err != nil {
		return fmt.Errorf("error extracting kernel: %w", err)
	}

	b.kernelTempPath = tempF.Name()

	return nil
}

func (b *BootLoader) extractInitrd(label string) error {
	path := filepath.Join("/", label, constants.InitramfsAsset)

	r, err := b.bootFs.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open initrd %q: %w", path, err)
	}

	tempF, err := ioutil.TempFile("", "talos")
	if err != nil {
		return fmt.Errorf("error creating temporary initrd file: %w", err)
	}

	defer tempF.Close() //nolint:errcheck

	if _, err = io.Copy(tempF, r); err != nil {
		return fmt.Errorf("error extracting initrd: %w", err)
	}

	b.initrdTempPath = tempF.Name()

	return nil
}
