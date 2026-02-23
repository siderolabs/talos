// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/siderolabs/go-cmd/pkg/cmd"
	"golang.org/x/sys/unix"
)

const (
	// FilesystemTypeXFS is the filesystem type for XFS.
	FilesystemTypeXFS = "xfs"
)

// XFSGrow expands a XFS filesystem to the maximum possible. The partition
// MUST be mounted, or this will fail.
func XFSGrow(ctx context.Context, partname string) error {
	_, err := cmd.RunWithOptions(ctx, "xfs_growfs", []string{"-d", partname})
	if err != nil {
		return fmt.Errorf("failed to grow XFS filesystem: %w", err)
	}

	return err
}

// XFSRepair repairs a XFS filesystem on the specified partition.
func XFSRepair(ctx context.Context, partname string) error {
	_, err := cmd.RunWithOptions(ctx, "xfs_repair", []string{partname})
	if err != nil {
		return fmt.Errorf("error repairing XFS filesystem: %w", err)
	}

	return nil
}

// XFS creates a XFS filesystem on the specified partition.
//
//nolint:gocyclo
func XFS(ctx context.Context, partname string, setters ...Option) error {
	if partname == "" {
		return errors.New("missing path to disk")
	}

	opts := NewDefaultOptions(setters...)

	// The ftype=1 naming option is required by overlayfs.
	args := []string{"-n", "ftype=1"}

	if opts.ConfigFile != "" {
		args = append(args, "-c", fmt.Sprintf("options=%s", opts.ConfigFile))
	}

	if opts.Force {
		args = append(args, "-f")
	}

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	if opts.UnsupportedFSOption {
		args = append(args, "--unsupported")
	}

	if opts.SourceDirectory != "" {
		r, err := GenerateProtofile(opts.SourceDirectory)
		if err != nil {
			return fmt.Errorf("failed to generate protofile: %w", err)
		}

		fd, err := unix.MemfdCreate("protofile", 0)
		if err != nil {
			return fmt.Errorf("error creating memfd for protofile: %w", err)
		}

		protoMemfd := os.NewFile(uintptr(fd), "protofile")
		defer protoMemfd.Close() //nolint:errcheck

		_, err = io.Copy(protoMemfd, r)
		if err != nil {
			return fmt.Errorf("failed to write protofile to memfd: %w", err)
		}

		// Seek back to the beginning so mkfs.xfs can read from start
		if _, err := protoMemfd.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to seek protofile: %w", err)
		}

		args = append(args, "-p", fmt.Sprintf("file=/proc/self/fd/%d", protoMemfd.Fd()))
	}

	if opts.Reproducible {
		if opts.Label == "" {
			return errors.New("label must be set for reproducible XFS filesystem")
		}

		partitionGUID := GUIDFromLabel(opts.Label)

		args = append(args, "-m", fmt.Sprintf("uuid=%s", partitionGUID.String()))
	}

	args = append(args, partname)

	opts.Printf("creating xfs filesystem on %s with args: %v", partname, args)

	_, err := cmd.RunWithOptions(ctx, "mkfs.xfs", args)

	return err
}
