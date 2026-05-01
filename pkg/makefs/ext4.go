// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	errCh := make(chan error, 1)
	numProcs := 1

	var (
		pr         *os.File
		pw         *os.File
		stdErrBuf1 bytes.Buffer
		stdErrBuf2 bytes.Buffer
	)

	if opts.SourceDirectory != "" {
		args = append(args, "-d", "-")
	}

	args = append(args, partname)

	opts.Printf("creating ext4 filesystem on %s with args: %v", partname, args)

	cmdMkfs := exec.CommandContext(ctx, "mkfs.ext4", args...)
	cmdMkfs.Stderr = &stdErrBuf1

	// Use a tar archive instead of passing a source directory directly to mke2fs.
	// This allows us to force all files in the image to be owned by root (uid/gid 0)
	// regardless of their ownership on the host, enabling rootless operation without
	// requiring elevated privileges.
	if opts.SourceDirectory != "" {
		var err error

		pr, pw, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe for tar archive: %w", err)
		}

		defer pr.Close() //nolint:errcheck
		defer pw.Close() //nolint:errcheck

		cmdMkfs.Stdin = pr

		timestamp, ok, err := utils.SourceDateEpoch()
		if err != nil {
			return fmt.Errorf("failed to get SOURCE_DATE_EPOCH: %w", err)
		}

		if !ok {
			timestamp = time.Now().Unix()
		}

		cmdTar := exec.CommandContext(
			ctx,
			"tar",
			"-cf",
			"-",
			"-C",
			opts.SourceDirectory,
			"--sort=name",
			"--owner=0",
			"--group=0",
			"--numeric-owner",
			"--mtime=@"+strconv.FormatInt(timestamp, 10),
			".",
		)
		cmdTar.Stdout = pw
		cmdTar.Stderr = &stdErrBuf2

		if err := cmdTar.Start(); err != nil {
			return fmt.Errorf("failed to start tar command: %w", err)
		}

		if err := pw.Close(); err != nil {
			return fmt.Errorf("failed to close pipe writer: %w", err)
		}

		numProcs++

		go func() {
			errCh <- cmdTar.Wait()
		}()
	}

	if err := cmdMkfs.Start(); err != nil {
		return fmt.Errorf("failed to start mkfs.ext4: %w", err)
	}

	if pr != nil {
		if err := pr.Close(); err != nil {
			return fmt.Errorf("failed to close pipe reader: %w", err)
		}
	}

	go func() {
		errCh <- cmdMkfs.Wait()
	}()

	var runErr error

	for range numProcs {
		if err := <-errCh; err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("command failed: %w", err))
		}
	}

	if runErr != nil {
		runErr = fmt.Errorf("failed to create ext4 filesystem: %w:\n%s\n%s", runErr, stdErrBuf1.String(), stdErrBuf2.String())
	}

	return runErr
}

// Ext4Resize expands a ext4 filesystem to the maximum possible.
func Ext4Resize(ctx context.Context, partname string) error {
	// resizing the filesystem requires a check first
	if err := Ext4Repair(ctx, partname); err != nil {
		return fmt.Errorf("failed to repair before growing ext4 filesystem: %w", err)
	}

	_, err := cmd.RunWithOptions(ctx, "resize2fs", []string{partname})
	if err != nil {
		return fmt.Errorf("failed to grow ext4 filesystem: %w", err)
	}

	return nil
}

// Ext4Repair repairs a ext4 filesystem.
func Ext4Repair(ctx context.Context, partname string) error {
	_, err := cmd.RunWithOptions(ctx, "e2fsck", []string{"-f", "-p", partname})
	if err != nil {
		return fmt.Errorf("failed to repair ext4 filesystem: %w", err)
	}

	return nil
}
