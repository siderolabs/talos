// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

const (
	// FilesystemTypeBTRFS is the filesystem type for BTRFS.
	FilesystemTypeBTRFS = "btrfs"
)

// BTRFSGrow expands a BTRFS filesystem to the maximum possible.
//
// The partition  MUST be mounted, and partname must be a path within the mount (typically the
// mount point), since btrfs filesystem resize operates on a mounted filesystem.
func BTRFSGrow(ctx context.Context, partname string) error {
	_, err := cmd.RunWithOptions(ctx, "btrfs", []string{"filesystem", "resize", "max", partname})
	if err != nil {
		return fmt.Errorf("failed to grow BTRFS filesystem: %w", err)
	}

	return nil
}

// BTRFSRepair repairs a BTRFS filesystem on the specified partition.
//
// As this operation is potentially dangerous, we are going to wire into
// the volume mount flow, but we will keep it here for completeness.
//
// The filesystem must NOT be mounted. --force is passed to skip the interactive
// 10-second warning delay; --repair on a damaged filesystem is dangerous and
// can make things worse — callers should ensure a check is warranted.
func BTRFSRepair(ctx context.Context, partname string) error {
	_, err := cmd.RunWithOptions(ctx, "btrfs", []string{"check", "--repair", "--force", partname})
	if err != nil {
		return fmt.Errorf("error repairing BTRFS filesystem: %w", err)
	}

	return nil
}

// BTRFS creates a btrfs filesystem on the specified partition.
func BTRFS(ctx context.Context, partname string, setters ...Option) error {
	if partname == "" {
		return errors.New("missing path to disk")
	}

	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Force {
		args = append(args, "-f")
	}

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	if opts.SectorSize > 0 {
		args = append(args, "-s", strconv.FormatUint(uint64(opts.SectorSize), 10))
	}

	if opts.Reproducible {
		if opts.Label == "" {
			return errors.New("label must be set for reproducible BTRFS filesystem")
		}

		partitionGUID := GUIDFromLabel(opts.Label)

		// Btrfs embeds randomly-generated UUIDs in every tree block header, so
		// mkfs.btrfs output is not byte-identical even with -U/--device-uuid
		// pinned. Setting both still pins the externally-visible identifiers
		// (fsid and dev_item.uuid), which is what callers typically rely on.
		args = append(args, "-U", partitionGUID.String(), "--device-uuid", partitionGUID.String())
	}

	if opts.SourceDirectory != "" {
		args = append(args, "-r", opts.SourceDirectory)
	}

	args = append(args, partname)

	opts.Printf("creating btrfs filesystem on %s with args: %v", partname, args)

	_, err := cmd.RunWithOptions(ctx, "mkfs.btrfs", args)
	if err != nil {
		return fmt.Errorf("failed to create btrfs filesystem: %w", err)
	}

	return nil
}
