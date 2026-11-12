// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package extconfigroot mounts the tmpfs which holds rendered extension service config files.
package extconfigroot

import (
	"errors"
	"fmt"
	"os"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

// Setup mounts a detached tmpfs and binds it read-only at target.
//
// The returned root is the only writable view.
func Setup(target string, logger *zap.Logger) (xfs.Root, func() error, error) {
	fsOptions := []fsopen.Option{
		fsopen.WithStringParameter("mode", "0755"),
		fsopen.WithStringParameter("size", "1M"),
	}

	if selinux.IsEnabled() {
		fsOptions = append(fsOptions, fsopen.WithStringParameter("context", constants.SystemSelinuxLabel))
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create extension service config mount target %q: %w", target, err)
	}

	mgr := mountv3.NewManager(
		mountv3.WithDetached(),
		mountv3.WithPrinter(logger.Sugar().Infof),
		mountv3.WithFsopen("tmpfs", fsOptions...),
	)

	point, err := mgr.Mount()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to mount anonymous tmpfs for extension service config: %w", err)
	}

	cleanup := func() {
		mgr.Unmount() //nolint:errcheck
	}

	root := point.Root()

	rootFD, err := root.Fd()
	if err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("failed to get extension service config root fd: %w", err)
	}

	if err = mountv3.BindReadonlyFd(rootFD, target); err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("failed to bind extension service config root at %q: %w", target, err)
	}

	logger.Info("mounted extension service config tmpfs", zap.String("target", target))

	unmount := func() error {
		return errors.Join(
			unix.Unmount(target, unix.MNT_DETACH),
			mgr.Unmount(),
		)
	}

	return root, unmount, nil
}
