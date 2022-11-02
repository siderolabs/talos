// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// prepareRootfs creates /system/libexec/<service> rootfs and bind-mounts /sbin/init there.
func prepareRootfs(id string) error {
	rootfsPath := filepath.Join(constants.SystemLibexecPath, id)

	if err := os.MkdirAll(rootfsPath, 0o711); err != nil { // rwx--x--x, non-root programs should be able to follow path
		return fmt.Errorf("failed to create rootfs %q: %w", rootfsPath, err)
	}

	executablePath := filepath.Join(rootfsPath, id)

	if err := os.WriteFile(executablePath, nil, 0o555); err != nil { // r-xr-xr-x, non-root programs should be able to execute & read
		return fmt.Errorf("failed to create empty executable %q: %w", executablePath, err)
	}

	if err := unix.Mount("/sbin/init", executablePath, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to create bind mount for %q: %w", executablePath, err)
	}

	return nil
}
