// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/talos-systems/talos/internal/pkg/kernel/vmlinuz"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/table/gpt"
	"github.com/talos-systems/talos/pkg/constants"
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
func (b *BootLoader) ExtractAssets() (BootAssets, error) {
	if err := b.findBootPartition(); err != nil {
		return BootAssets{}, err
	}

	if err := b.openFilesystem(); err != nil {
		return BootAssets{}, err
	}

	if err := b.extractKernel(); err != nil {
		return BootAssets{}, err
	}

	if err := b.extractInitrd(); err != nil {
		return BootAssets{}, err
	}

	return BootAssets{
		KernelPath: b.kernelTempPath,
		InitrdPath: b.initrdTempPath,
	}, nil
}

// Close the bootloader.
func (b *BootLoader) Close() error {
	if b.kernelTempPath != "" {
		os.Remove(b.kernelTempPath) //nolint: errcheck
		b.kernelTempPath = ""
	}

	if b.initrdTempPath != "" {
		os.Remove(b.initrdTempPath) //nolint: errcheck
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
	diskTable, err := gpt.NewGPT("vda", b.diskF)
	if err != nil {
		return fmt.Errorf("error creating GPT object: %w", err)
	}

	if err = diskTable.Read(); err != nil {
		return fmt.Errorf("error reading GPT: %w", err)
	}

	var bootPartition table.Partition

	for _, part := range diskTable.Partitions() {
		// TODO: should we do better matching here
		if part.No() == 1 {
			bootPartition = part
			break
		}
	}

	if bootPartition == nil {
		return fmt.Errorf("no boot partition found")
	}

	b.bootPartitionReader = io.NewSectionReader(b.diskF, bootPartition.Start()*diskImageSectorSize, bootPartition.Length()*diskImageSectorSize)

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

func (b *BootLoader) extractKernel() error {
	r, err := b.bootFs.Open(filepath.Join("default", constants.KernelAsset))
	if err != nil {
		return fmt.Errorf("error opening kernel asset: %w", err)
	}

	kernelR, err := vmlinuz.Decompress(bufio.NewReader(r))
	if err != nil {
		return fmt.Errorf("error decompressing kernel: %w", err)
	}

	defer kernelR.Close() //nolint: errcheck

	tempF, err := ioutil.TempFile("", "talos")
	if err != nil {
		return fmt.Errorf("error creating temporary kernel image file: %w", err)
	}

	defer tempF.Close() //nolint: errcheck

	if _, err = io.Copy(tempF, kernelR); err != nil {
		return fmt.Errorf("error extracting kernel: %w", err)
	}

	b.kernelTempPath = tempF.Name()

	return nil
}

func (b *BootLoader) extractInitrd() error {
	r, err := b.bootFs.Open(filepath.Join("default", constants.InitramfsAsset))
	if err != nil {
		return fmt.Errorf("error opening initrd: %w", err)
	}

	tempF, err := ioutil.TempFile("", "talos")
	if err != nil {
		return fmt.Errorf("error creating temporary initrd file: %w", err)
	}

	defer tempF.Close() //nolint: errcheck

	if _, err = io.Copy(tempF, r); err != nil {
		return fmt.Errorf("error extracting initrd: %w", err)
	}

	b.initrdTempPath = tempF.Name()

	return nil
}
