// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package partition provides common utils for system partition format.
package partition

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/talos-systems/go-blockdevice/blockdevice"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/makefs"
)

// FormatOptions contains format parameters.
type FormatOptions struct {
	Label          string
	PartitionType  Type
	FileSystemType FileSystemType
	Size           uint64
	Force          bool
}

// NewFormatOptions creates a new format options.
func NewFormatOptions(label string) *FormatOptions {
	opts, ok := systemPartitions[label]
	if ok {
		return &opts
	}

	return nil
}

// Format zeroes the device and formats it using filesystem type provided.
func Format(devname string, t *FormatOptions) error {
	if t.FileSystemType == FilesystemTypeNone {
		return zeroPartition(devname, int64(t.Size))
	}

	opts := []makefs.Option{makefs.WithForce(t.Force), makefs.WithLabel(t.Label)}
	log.Printf("formatting the partition %q as %q with label %q\n", devname, t.FileSystemType, t.Label)

	switch t.FileSystemType {
	case FilesystemTypeVFAT:
		return makefs.VFAT(devname, opts...)
	case FilesystemTypeXFS:
		return makefs.XFS(devname, opts...)
	default:
		return fmt.Errorf("unsupported filesystem type: %q", t.FileSystemType)
	}
}

// zeroPartition fills the partition with zeroes.
func zeroPartition(devname string, size int64) (err error) {
	log.Printf("zeroing out %q", devname)

	zeroes, err := os.Open("/dev/zero")
	if err != nil {
		return err
	}

	defer zeroes.Close() //nolint:errcheck

	part, err := os.OpenFile(devname, os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	defer part.Close() //nolint:errcheck

	// wipe at least minimal header size
	if size == 0 {
		size = blockdevice.FastWipeRange
	}

	_, err = io.CopyN(part, zeroes, size)

	return err
}

var systemPartitions = map[string]FormatOptions{
	constants.EFIPartitionLabel: {
		Label:          constants.EFIPartitionLabel,
		PartitionType:  EFISystemPartition,
		FileSystemType: FilesystemTypeVFAT,
		Size:           EFISize,
		Force:          true,
	},
	constants.BIOSGrubPartitionLabel: {
		Label:          constants.BIOSGrubPartitionLabel,
		PartitionType:  BIOSBootPartition,
		FileSystemType: FilesystemTypeNone,
		Size:           BIOSGrubSize,
		Force:          true,
	},
	constants.BootPartitionLabel: {
		Label:          constants.BootPartitionLabel,
		PartitionType:  LinuxFilesystemData,
		FileSystemType: FilesystemTypeXFS,
		Size:           BootSize,
		Force:          true,
	},
	constants.MetaPartitionLabel: {
		Label:          constants.MetaPartitionLabel,
		PartitionType:  LinuxFilesystemData,
		FileSystemType: FilesystemTypeNone,
		Size:           MetaSize,
		Force:          true,
	},
	constants.StatePartitionLabel: {
		Label:          constants.StatePartitionLabel,
		PartitionType:  LinuxFilesystemData,
		FileSystemType: FilesystemTypeXFS,
		Size:           StateSize,
		Force:          true,
	},
	constants.EphemeralPartitionLabel: {
		Label:          constants.EphemeralPartitionLabel,
		PartitionType:  LinuxFilesystemData,
		FileSystemType: FilesystemTypeXFS,
		Size:           0,
		Force:          true,
	},
}
