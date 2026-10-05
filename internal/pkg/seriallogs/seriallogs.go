// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package seriallogs streams read-only serial capture files independently of the console.
package seriallogs

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"time"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	chunkSize   = 32 * 1024
	maxTailScan = 8 * 1024 * 1024
)

// Stream sends an active-file snapshot, optionally following append, detectable
// truncation, and replacement at 100 ms intervals. It drains the prior inode
// before switching but cannot recover rotations outrunning retention or writes
// to the old inode after switching. send must consume data before returning and
// honor ctx when blocking. No goroutines, source writes, or console leases are used.
// A nonnegative tail counts newline-delimited lines, including an unterminated
// last line, and scans at most 8 MiB. Paths must be selected by a trusted caller.
func Stream(ctx context.Context, path string, tail int32, follow bool, send func([]byte) error) (retErr error) {
	if err := ctx.Err(); err != nil {
		return status.FromContextError(err).Err()
	}

	file, err := open(path)
	if err != nil {
		return fileError(err)
	}

	cursor := fileCursor{
		file: file,
	}
	defer func() { retErr = errors.Join(retErr, cursor.file.Close()) }()

	info, err := file.Stat()
	if err != nil {
		return fileError(err)
	}

	cursor.offset, err = tailOffset(ctx, file, info.Size(), tail)
	if err != nil {
		return err
	}

	if err = cursor.drain(ctx, info.Size(), send); err != nil || !follow {
		return err
	}

	return cursor.follow(ctx, path, send)
}

func (c *fileCursor) follow(ctx context.Context, path string, send func([]byte) error) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		case <-ticker.C:
		}

		if err := c.poll(ctx, path, send); err != nil {
			return err
		}
	}
}

type fileCursor struct {
	file   *os.File
	offset int64
}

func (c *fileCursor) poll(ctx context.Context, path string, send func([]byte) error) error {
	info, err := c.file.Stat()
	if err != nil {
		return fileError(err)
	}

	if info.Size() < c.offset {
		c.offset = 0
	}

	if err = c.drain(ctx, info.Size(), send); err != nil {
		return err
	}

	next, err := open(path)
	if errors.Is(err, os.ErrNotExist) {
		// Keep reading the prior inode across the writer's rename/create window.
		return nil
	}

	if err != nil {
		return fileError(err)
	}

	nextInfo, err := next.Stat()
	if err != nil {
		return errors.Join(fileError(err), next.Close())
	}

	if os.SameFile(info, nextInfo) {
		return next.Close()
	}

	return c.replace(ctx, next, nextInfo.Size(), send)
}

func (c *fileCursor) replace(ctx context.Context, next *os.File, size int64, send func([]byte) error) error {
	// Drain one bounded snapshot of the old inode before releasing it. This is
	// best-effort rotation following, not an exactly-once replay protocol.
	info, err := c.file.Stat()
	if err == nil {
		err = c.drain(ctx, info.Size(), send)
	}

	closeErr := c.file.Close()
	c.file = next
	c.offset = 0

	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}

	return c.drain(ctx, size, send)
}

func open(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	file := os.NewFile(uintptr(fd), path)

	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}

	if !info.Mode().IsRegular() {
		return nil, errors.Join(status.Error(codes.FailedPrecondition, "serial log is not a regular file"), file.Close())
	}

	return file, nil
}

func fileError(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return status.Error(codes.NotFound, "serial log file is absent")
	case errors.Is(err, os.ErrPermission):
		return status.Error(codes.PermissionDenied, "serial log file is not readable")
	case errors.Is(err, unix.ELOOP):
		return status.Error(codes.FailedPrecondition, "serial log must not be a symbolic link")
	default:
		if _, ok := status.FromError(err); ok {
			return err
		}

		return status.Errorf(codes.Internal, "read serial log: %v", err)
	}
}

func tailOffset(ctx context.Context, file *os.File, size int64, tail int32) (int64, error) {
	if tail < 0 {
		return 0, nil
	}

	if tail == 0 {
		return size, nil
	}

	buffer := make([]byte, chunkSize)

	for end := size; end > 0; {
		if err := ctx.Err(); err != nil {
			return 0, status.FromContextError(err).Err()
		}

		start := max(int64(0), end-int64(len(buffer)))

		n, err := file.ReadAt(buffer[:end-start], start)
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, fileError(err)
		}

		if offset, found := scanTail(buffer[:n], start, size, &tail); found {
			return offset, nil
		}

		if size-start >= maxTailScan && start > 0 {
			return 0, status.Error(codes.ResourceExhausted, "serial log tail exceeds 8 MiB scan limit")
		}

		end = start
	}

	return 0, nil
}

func scanTail(buffer []byte, start, size int64, tail *int32) (int64, bool) {
	for i, value := range slices.Backward(buffer) {
		if value != '\n' || start+int64(i) == size-1 {
			continue
		}

		*tail--
		if *tail == 0 {
			return start + int64(i) + 1, true
		}
	}

	return 0, false
}

func (c *fileCursor) drain(ctx context.Context, end int64, send func([]byte) error) error {
	reader := io.NewSectionReader(c.file, c.offset, max(int64(0), end-c.offset))
	buffer := make([]byte, chunkSize)

	for {
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}

		n, err := reader.Read(buffer)
		if n > 0 {
			if sendErr := send(buffer[:n]); sendErr != nil {
				return sendErr
			}

			c.offset += int64(n)
		}

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fileError(err)
		}
	}
}
