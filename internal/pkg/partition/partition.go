// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package partition provides common utils for system partition format.
package partition

import (
	"github.com/dustin/go-humanize"
	"github.com/siderolabs/go-blockdevice/blockdevice/partition/gpt"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Options contains the options for creating a partition.
type Options struct {
	PartitionLabel     string
	PartitionType      Type
	Size               uint64
	LegacyBIOSBootable bool
}

// NewPartitionOptions returns a new PartitionOptions.
func NewPartitionOptions(label string, uki bool) *Options {
	return systemPartitionsPartitonOptions(label, uki)
}

// Locate existing partition on the disk by label.
func Locate(pt *gpt.GPT, label string) (*gpt.Partition, error) {
	for _, part := range pt.Partitions().Items() {
		if part.Name == label {
			return part, nil
		}
	}

	return nil, nil
}

// Partition creates a new partition on the specified device.
// Returns the path to the newly created partition.
func Partition(pt *gpt.GPT, pos int, device string, partitionOpts Options, printf func(string, ...any)) (string, error) {
	printf("partitioning %s - %s %q\n", device, partitionOpts.PartitionLabel, humanize.Bytes(partitionOpts.Size))

	opts := []gpt.PartitionOption{
		gpt.WithPartitionType(partitionOpts.PartitionType),
		gpt.WithPartitionName(partitionOpts.PartitionLabel),
	}

	if partitionOpts.Size == 0 {
		opts = append(opts, gpt.WithMaximumSize(true))
	}

	if partitionOpts.LegacyBIOSBootable {
		opts = append(opts, gpt.WithLegacyBIOSBootableAttribute(true))
	}

	part, err := pt.InsertAt(pos, partitionOpts.Size, opts...)
	if err != nil {
		return "", err
	}

	partitionName, err := part.Path()
	if err != nil {
		return "", err
	}

	printf("created %s (%s) size %d blocks", partitionName, partitionOpts.PartitionLabel, part.Length())

	return partitionName, nil
}

func systemPartitionsPartitonOptions(label string, uki bool) *Options {
	switch label {
	case constants.EFIPartitionLabel:
		partitionOptions := &Options{
			PartitionType: EFISystemPartition,
			Size:          EFISize,
		}

		if uki {
			partitionOptions.Size = EFIUKISize
		}

		return partitionOptions
	case constants.BIOSGrubPartitionLabel:
		if uki {
			panic("BIOS partition is not supported with UKI")
		}

		return &Options{
			PartitionType: BIOSBootPartition,
			Size:          BIOSGrubSize,
		}
	case constants.BootPartitionLabel:
		if uki {
			panic("BOOT partition is not supported with UKI")
		}

		return &Options{
			PartitionType: LinuxFilesystemData,
			Size:          BootSize,
		}
	case constants.MetaPartitionLabel:
		return &Options{
			PartitionType: LinuxFilesystemData,
			Size:          MetaSize,
		}
	case constants.StatePartitionLabel:
		return &Options{
			PartitionType: LinuxFilesystemData,
			Size:          StateSize,
		}
	case constants.EphemeralPartitionLabel:
		return &Options{
			PartitionType: LinuxFilesystemData,
			Size:          0,
		}
	default:
		return nil
	}
}
