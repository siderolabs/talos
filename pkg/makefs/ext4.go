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
	"os/exec"
	"strconv"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/pkg/imager/utils"
)

const (
	// FilesystemTypeEXT4 is the filesystem type for EXT4.
	FilesystemTypeEXT4 = "ext4"
)

// Ext4 creates a ext4 filesystem on the specified partition.
//
//nolint:gocyclo
func Ext4(ctx context.Context, partname string, setters ...Option) error {
	if partname == "" {
		return errors.New("missing path to disk")
	}

	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	if opts.Force {
		args = append(args, "-F")
	}

	if opts.Reproducible {
		if opts.Label == "" {
			return errors.New("label must be set for reproducible ext4 filesystem")
		}

		partitionGUID := GUIDFromLabel(opts.Label)

		args = append(args, "-U", partitionGUID.String())
		args = append(args, "-E", fmt.Sprintf("hash_seed=%s", partitionGUID.String()))
	}

	var errCh chan error

	// Use a tar archive instead of passing a source directory directly to mke2fs.
	// This allows us to force all files in the image to be owned by root (uid/gid 0)
	// regardless of their ownership on the host, enabling rootless operation without
	// requiring elevated privileges.
	if opts.SourceDirectory != "" {
		errCh = make(chan error, 1)

		pr, pw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}

		defer pr.Close() //nolint:errcheck

		args = append(args, "-d", "-")

		ctx = cmd.WithStdin(ctx, pr)

		go func() {
			defer pw.Close() //nolint:errcheck

			errCh <- handleTarArchive(ctx, opts.SourceDirectory, pw)
		}()
	}

	args = append(args, partname)

	opts.Printf("creating ext4 filesystem on %s with args: %v", partname, args)

	_, err := cmd.RunContext(ctx, "mkfs.ext4", args...)
	if err != nil {
		return err
	}

	if errCh != nil {
		return <-errCh
	}

	return nil
}

// Ext4Resize expands a ext4 filesystem to the maximum possible.
func Ext4Resize(partname string) error {
	// resizing the filesystem requires a check first
	if err := Ext4Repair(partname); err != nil {
		return fmt.Errorf("failed to repair before growing ext4 filesystem: %w", err)
	}

	_, err := cmd.Run("resize2fs", partname)
	if err != nil {
		return fmt.Errorf("failed to grow ext4 filesystem: %w", err)
	}

	return nil
}

// Ext4Repair repairs a ext4 filesystem.
func Ext4Repair(partname string) error {
	_, err := cmd.Run("e2fsck", "-f", "-p", partname)
	if err != nil {
		return fmt.Errorf("failed to repair ext4 filesystem: %w", err)
	}

	return nil
}

// handleTarArchive creates a tar archive from the sourceDir and writes it to the provided
// io.WriteCloser.
func handleTarArchive(ctx context.Context, sourceDir string, w io.WriteCloser) error {
	timestamp, ok, err := utils.SourceDateEpoch()
	if err != nil {
		return fmt.Errorf("failed to get SOURCE_DATE_EPOCH: %w", err)
	}

	if !ok {
		timestamp = time.Now().Unix()
	}

	cmd := exec.CommandContext(
		ctx,
		"tar",
		"-cf",
		"-",
		"-C",
		sourceDir,
		"--sort=name",
		"--owner=0",
		"--group=0",
		"--numeric-owner",
		"--mtime=@"+strconv.FormatInt(timestamp, 10),
		".",
	)

	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tar command: %w", err)
	}

	closeErr := w.Close()
	waitErr := cmd.Wait()

	if closeErr != nil {
		return fmt.Errorf("failed to close pipe writer: %w", closeErr)
	}

	return waitErr
}
