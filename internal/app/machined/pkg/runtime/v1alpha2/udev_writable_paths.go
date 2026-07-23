// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

const (
	hwdbFilename     = "hwdb.bin"
	hwdbSelinuxLabel = "system_u:object_r:udev_hwdb_t:s0"
)

func setupUdevWritablePaths(udevPath string, logger *zap.Logger) (func() error, error) {
	printer := logger.Sugar().Infof

	fsOptions := []fsopen.Option{
		fsopen.WithStringParameter("mode", "0750"),
	}

	mgr := mountv3.NewManager(
		mountv3.WithDetached(),
		mountv3.WithPrinter(printer),
		mountv3.WithFsopen("tmpfs", fsOptions...),
	)

	point, err := mgr.Mount()
	if err != nil {
		return nil, fmt.Errorf("failed to mount tmpfs at %q: %w", udevPath, err)
	}

	sourcePath := filepath.Join(udevPath, hwdbFilename)

	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", sourcePath, err)
	}

	hwdbRoot := point.Root()

	if err = xfs.WriteFile(hwdbRoot, hwdbFilename, contents, 0o640); err != nil {
		return nil, fmt.Errorf("failed to seed udev hwdb: %w", err)
	}

	if err = selinux.FSetLabel(hwdbRoot, hwdbFilename, hwdbSelinuxLabel); err != nil {
		return nil, fmt.Errorf("failed to label udev hwdb: %w", err)
	}

	if err = mountv3.BindRootPath(hwdbRoot, hwdbFilename, sourcePath, unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NODEV|unix.MOUNT_ATTR_NOEXEC); err != nil {
		return nil, fmt.Errorf("failed to bind udev hwdb: %w", err)
	}

	printer("mounted writable udev hwdb at %s", sourcePath)

	unmount := func() error {
		return errors.Join(
			unix.Unmount(filepath.Join(udevPath, hwdbFilename), unix.MNT_DETACH),
			mgr.Unmount(),
		)
	}

	return unmount, nil
}
