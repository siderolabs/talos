// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"fmt"
	"os"
	"path/filepath"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/siderolabs/talos/internal/pkg/containermode"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// prepareRootfs creates /system/libexec/<service> rootfs and bind-mounts /sbin/init there.
func prepareRootfs(id string) error {
	rootfsPath := filepath.Join(constants.SystemLibexecPath, id)

	if err := os.MkdirAll(rootfsPath, 0o711); err != nil { // rwx--x--x, non-root programs should be able to follow path
		return fmt.Errorf("failed to create rootfs %q: %w", rootfsPath, err)
	}

	return nil
}

// bindMountContainerMarker bind-mounts a file used for container detection into a container service.
func bindMountContainerMarker(mounts []specs.Mount) []specs.Mount {
	if containermode.InContainer() {
		mounts = append(
			mounts,
			specs.Mount{Type: "bind", Destination: constants.ContainerMarkerFilePath, Source: constants.ContainerMarkerFilePath, Options: []string{"bind", "ro"}},
		)
	}

	return mounts
}
