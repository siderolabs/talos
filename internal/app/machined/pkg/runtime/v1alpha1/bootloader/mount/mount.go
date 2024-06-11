// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mount provides bootloader mount operations.
package mount

import (
	"fmt"

	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-blockdevice/v2/partitioning"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/internal/pkg/mount/v2"
)

// Spec specifies what has to be mounted.
type Spec struct {
	PartitionLabel string

	FilesystemType string

	MountTarget string
}

// NotFoundTag is a tag for a partition not found/mismatch errors.
type NotFoundTag struct{}

// PartitionOp mounts specified partitions with the specified label, executes the operation func, and unmounts the partition(s).
func PartitionOp(
	disk string, specs []Spec, opFunc func() error,
	probeOptions []blkid.ProbeOption,
	newPointOptions []mount.NewPointOption,
	mountOptions []mount.OperationOption,
	info *blkid.Info, // might be nil
) error {
	if info == nil {
		var err error

		info, err = blkid.ProbePath(disk, probeOptions...)
		if err != nil {
			return fmt.Errorf("error probing disk %s: %w", disk, err)
		}
	}

	var points mount.Points

	for _, spec := range specs {
		var found bool

		for _, partition := range info.Parts {
			if pointer.SafeDeref(partition.PartitionLabel) == spec.PartitionLabel {
				if partition.Name != spec.FilesystemType {
					return xerrors.NewTaggedf[NotFoundTag]("partition %d with label %s is not of type %s (actual %q)", partition.PartitionIndex, *partition.PartitionLabel, spec.FilesystemType, partition.Name)
				}

				points = append(points,
					mount.NewPoint(
						partitioning.DevName(disk, partition.PartitionIndex),
						spec.MountTarget,
						spec.FilesystemType,
						newPointOptions...,
					),
				)

				found = true

				break
			}
		}

		if !found {
			return xerrors.NewTaggedf[NotFoundTag]("partition with label %s not found", spec.PartitionLabel)
		}
	}

	unmounter, err := points.Mount(mountOptions...)
	if err != nil {
		return fmt.Errorf("error mounting partitions: %w", err)
	}

	defer unmounter() //nolint:errcheck

	return opFunc()
}
