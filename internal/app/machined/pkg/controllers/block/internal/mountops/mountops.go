// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mountops contains helpers for mount operations.
package mountops

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// CreateMountPoint creates the mount point directory if it doesn't exist.
//
// The directory is created with the ownership, permissions and SELinux label of the mount spec,
// and the return value indicates whether the directory was created.
func CreateMountPoint(target string, mountSpec block.MountSpec) (bool, error) {
	_, err := os.Lstat(target)
	if err == nil {
		return false, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("failed to stat mount point %q: %w", target, err)
	}

	if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, fmt.Errorf("failed to create parent directory of mount point %q: %w", target, err)
	}

	mode := cmp.Or(mountSpec.FileMode, 0o755)

	if err = os.Mkdir(target, mode); err != nil {
		return false, fmt.Errorf("failed to create mount point %q: %w", target, err)
	}

	if err = setupMountPoint(target, mode, mountSpec); err != nil {
		os.Remove(target) //nolint:errcheck

		return false, err
	}

	return true, nil
}

func setupMountPoint(target string, mode fs.FileMode, mountSpec block.MountSpec) error {
	// os.Mkdir is subject to umask
	if err := os.Chmod(target, mode); err != nil {
		return fmt.Errorf("failed to chmod mount point %q: %w", target, err)
	}

	if err := os.Lchown(target, mountSpec.UID, mountSpec.GID); err != nil {
		return fmt.Errorf("failed to chown mount point %q: %w", target, err)
	}

	// the filesystem of the mount point is the parent filesystem, which is not known here
	return mount.FilterSelinuxLabelErrors(target, "", selinux.SetLabel(target, mountSpec.SelinuxLabel))
}

// RemoveMountPoint removes the mount point directory created by createMountPoint after unmount.
//
// os.Remove never removes a non-empty directory.
func RemoveMountPoint(logger *zap.Logger, target string) {
	err := os.Remove(target)

	switch {
	case err == nil:
		logger.Info("removed mount point", zap.String("target", target))
	case !errors.Is(err, fs.ErrNotExist):
		logger.Warn("failed to remove mount point", zap.String("target", target), zap.Error(err))
	}
}

// CleanupEmptyDirectories removes empty directories directly under the given path.
//
// It is used to clean up mount points which were left behind, e.g. due to an unclean shutdown.
func CleanupEmptyDirectories(logger *zap.Logger, path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		logger.Warn("failed to read directory for cleanup", zap.String("path", path), zap.Error(err))

		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		target := filepath.Join(path, entry.Name())

		if err = os.Remove(target); err != nil {
			if !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) && !errors.Is(err, syscall.EBUSY) {
				logger.Warn("failed to remove empty directory", zap.String("path", target), zap.Error(err))
			}

			continue
		}

		logger.Info("removed empty directory", zap.String("path", target))
	}
}
