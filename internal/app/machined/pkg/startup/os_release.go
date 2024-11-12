// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package startup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// OSRelease renders a valid /etc/os-release file and writes it to disk.
//
// The node's OS Image field is reported by the node from /etc/os-release.
func OSRelease(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	if err := createBindMount(filepath.Join(constants.SystemEtcPath, "os-release"), "/etc/os-release"); err != nil {
		return err
	}

	contents, err := version.OSRelease()
	if err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(constants.SystemEtcPath, "os-release"), contents, 0o644); err != nil {
		return fmt.Errorf("failed to write os-release: %w", err)
	}

	return next()(ctx, log, rt, next)
}

// createBindMount creates a common way to create a writable source file with a
// bind mounted destination. This is most commonly used for well known files
// under /etc that need to be adjusted during startup.
func createBindMount(src, dst string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(src, os.O_WRONLY|os.O_CREATE, 0o644); err != nil {
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	if err = unix.Mount(src, dst, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to create bind mount for %s: %w", dst, err)
	}

	return nil
}
