// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func trySyncMount(target string) error {
	// Try the directory path first, then fall back to a file mountpoint.
	fd, err := unix.Open(target, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ENOTDIR) {
			fd, err = unix.Open(target, unix.O_RDONLY|unix.O_CLOEXEC, 0)
			if err != nil {
				return fmt.Errorf("open %q as file: %w", target, err)
			}
		} else {
			return fmt.Errorf("open %q as directory: %w", target, err)
		}
	}
	defer unix.Close(fd) //nolint:errcheck

	// sync the filesystem backing this fd
	if err := unix.Syncfs(fd); err != nil {
		return fmt.Errorf("SYS_SYNCFS %q: %w", target, err)
	}

	return nil
}

func unmountLoop(ctx context.Context, printer func(string, ...any), target string, flags int, timeout time.Duration, extraMessage string) (bool, error) {
	errCh := make(chan error, 1)

	// we need to try to sync fs before
	if err := trySyncMount(target); err != nil {
		printer("sync failed: %s", err)
	}

	go func() {
		errCh <- unix.Unmount(target, flags)
	}()

	start := time.Now()

	progressTicker := time.NewTicker(timeout / 5)
	defer progressTicker.Stop()

unmountLoop:
	for {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case err := <-errCh:
			return true, err
		case <-progressTicker.C:
			timeLeft := timeout - time.Since(start)

			if timeLeft <= 0 {
				break unmountLoop
			}

			printer("unmounting %s%s is taking longer than expected, still waiting for %s", target, extraMessage, timeLeft)
		}
	}

	return false, nil
}

// SafeUnmount unmounts the target path, first without force, then with force if the first attempt fails.
//
// It makes sure that unmounting has a finite operation timeout.
// If recursive is true, it will first unmount all child mounts under target.
func SafeUnmount(ctx context.Context, printer func(string, ...any), target string, recursive bool) error {
	const (
		unmountTimeout      = 90 * time.Second
		unmountForceTimeout = 10 * time.Second
	)

	if printer == nil {
		printer = discard
	}

	if recursive {
		submounts, err := getSubmounts(target)
		if err != nil {
			printer("failed to get submounts for %s: %v", target, err)
		} else {
			for _, submount := range submounts {
				printer("recursively unmounting submount %s", submount)

				if err := safeUnmountSingle(ctx, printer, submount, unmountTimeout); err != nil {
					printer("failed to unmount submount %s: %v", submount, err)
				}
			}
		}
	}

	ok, err := unmountLoop(ctx, printer, target, 0, unmountTimeout, "")

	if ok {
		return err
	}

	printer("unmounting %s with force", target)

	ok, err = unmountLoop(ctx, printer, target, unix.MNT_FORCE, unmountForceTimeout, " with force flag")

	if ok {
		return err
	}

	return fmt.Errorf("unmounting %s with force flag timed out", target)
}

func safeUnmountSingle(ctx context.Context, printer func(string, ...any), target string, timeout time.Duration) error {
	ok, err := unmountLoop(ctx, printer, target, 0, timeout, "")
	if ok {
		return err
	}

	return nil
}

func logSubmounts(printer func(string, ...any), target string) {
	submounts, err := getSubmounts(target)
	if err != nil {
		printer("failed to get submounts for %s: %v", target, err)

		return
	}

	if len(submounts) > 0 {
		printer("submounts on %s: %v", target, submounts)
	}
}

// logMountUsers scans /proc to find processes that have open file descriptors,
// working directories, or memory-mapped files under the given mount target.
// This helps diagnose "device or resource busy" errors during unmount.
//
//nolint:gocyclo,cyclop
func logMountUsers(printer func(string, ...any), target string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		printer("failed to read /proc: %v", err)

		return
	}

	targetWithSlash := target + "/"

	found := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // not a PID directory
		}

		procPath := filepath.Join("/proc", entry.Name())

		var offendingPaths []string

		// Check cwd.
		if cwd, err := os.Readlink(filepath.Join(procPath, "cwd")); err == nil {
			if cwd == target || strings.HasPrefix(cwd, targetWithSlash) {
				offendingPaths = append(offendingPaths, "cwd="+cwd)
			}
		}

		// Check root.
		if root, err := os.Readlink(filepath.Join(procPath, "root")); err == nil {
			if root == target || strings.HasPrefix(root, targetWithSlash) {
				offendingPaths = append(offendingPaths, "root="+root)
			}
		}

		// Check open file descriptors.
		if fds, err := os.ReadDir(filepath.Join(procPath, "fd")); err == nil {
			for _, fd := range fds {
				if link, err := os.Readlink(filepath.Join(procPath, "fd", fd.Name())); err == nil {
					if link == target || strings.HasPrefix(link, targetWithSlash) {
						offendingPaths = append(offendingPaths, "fd/"+fd.Name()+"="+link)
					}
				}
			}
		}

		// Check memory-mapped files.
		if f, err := os.Open(filepath.Join(procPath, "maps")); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				// maps format: address perms offset dev inode pathname
				// pathname starts after the 5th field
				fields := strings.Fields(line)
				if len(fields) >= 6 {
					mappedPath := fields[len(fields)-1]
					if mappedPath == target || strings.HasPrefix(mappedPath, targetWithSlash) {
						offendingPaths = append(offendingPaths, "mmap="+mappedPath)
					}
				}
			}

			f.Close() //nolint:errcheck
		}

		if len(offendingPaths) == 0 {
			continue
		}

		found = true

		// Read process identity.
		comm := "<unknown>"

		if data, err := os.ReadFile(filepath.Join(procPath, "comm")); err == nil {
			comm = strings.TrimSpace(string(data))
		}

		cmdline := ""

		if data, err := os.ReadFile(filepath.Join(procPath, "cmdline")); err == nil {
			// cmdline uses null bytes as separators
			cmdline = strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", " ")
		}

		printer("mount %s held by pid %d (%s) cmdline=[%s]: %s",
			target, pid, comm, cmdline, strings.Join(offendingPaths, ", "))
	}

	if !found {
		printer("mount %s is busy but no processes found holding it (may be held by kernel references)", target)
	} else {
		printer("if you see this message, report a bug with the above information to help us identify and fix the issue")
	}
}
