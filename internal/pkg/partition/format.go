// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package partition provides common utils for system partition format.
package partition

import (
	"fmt"

	"github.com/siderolabs/go-blockdevice/blockdevice"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/makefs"
)

// FormatOptions contains format parameters.
type FormatOptions struct {
	Label               string
	FileSystemType      FileSystemType
	Force               bool
	UnsupportedFSOption bool
}

// NewFormatOptions creates a new format options.
func NewFormatOptions(label string) *FormatOptions {
	return systemPartitionsFormatOptions(label)
}

// Format zeroes the device and formats it using filesystem type provided.
func Format(devname string, t *FormatOptions, printf func(string, ...any)) error {
	if t.FileSystemType == FilesystemTypeNone {
		return zeroPartition(devname, printf)
	}

	opts := []makefs.Option{makefs.WithForce(t.Force), makefs.WithLabel(t.Label)}

	if t.UnsupportedFSOption {
		opts = append(opts, makefs.WithUnsupportedFSOption(t.UnsupportedFSOption))
	}

	printf("formatting the partition %q as %q with label %q\n", devname, t.FileSystemType, t.Label)

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
func zeroPartition(devname string, printf func(string, ...any)) (err error) {
	printf("zeroing out %q", devname)

	part, err := blockdevice.Open(devname, blockdevice.WithExclusiveLock(true))
	if err != nil {
		return err
	}

	defer part.Close() //nolint:errcheck

	return part.FastWipe()
}

func systemPartitionsFormatOptions(label string) *FormatOptions {
	switch label {
	case constants.EFIPartitionLabel:
		return &FormatOptions{
			Label:          constants.EFIPartitionLabel,
			FileSystemType: FilesystemTypeVFAT,
			Force:          true,
		}
	case constants.BIOSGrubPartitionLabel:
		return &FormatOptions{
			Label:          constants.BIOSGrubPartitionLabel,
			FileSystemType: FilesystemTypeNone,
			Force:          true,
		}
	case constants.BootPartitionLabel:
		return &FormatOptions{
			Label:          constants.BootPartitionLabel,
			FileSystemType: FilesystemTypeXFS,
			Force:          true,
		}
	case constants.MetaPartitionLabel:
		return &FormatOptions{
			Label:          constants.MetaPartitionLabel,
			FileSystemType: FilesystemTypeNone,
			Force:          true,
		}
	case constants.StatePartitionLabel:
		return &FormatOptions{
			Label:               constants.StatePartitionLabel,
			FileSystemType:      FilesystemTypeXFS,
			Force:               true,
			UnsupportedFSOption: true,
		}
	case constants.EphemeralPartitionLabel:
		return &FormatOptions{
			Label:          constants.EphemeralPartitionLabel,
			FileSystemType: FilesystemTypeXFS,
			Force:          true,
		}
	default:
		return nil
	}
}
