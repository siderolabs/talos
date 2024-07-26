// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/opencontainers/runc/libcontainer/dmz"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// RuncMemFDBindController created a locked memfd bind for the runc binary, so that it can be used instead of copying the actual runc binary everytime.
type RuncMemFDBindController struct {
	V1Alpha1Mode runtimetalos.Mode
}

// Name implements controller.Controller interface.
func (ctrl *RuncMemFDBindController) Name() string {
	return "cri.RuncMemFDBindController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RuncMemFDBindController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *RuncMemFDBindController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *RuncMemFDBindController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// This controller is only relevant in container mode.
	if ctrl.V1Alpha1Mode == runtimetalos.ModeContainer {
		return nil
	}

	runcPath := "/bin/runc"

	memfdFile, err := memfdClone(runcPath)
	if err != nil {
		return fmt.Errorf("memfd clone: %w", err)
	}
	defer memfdFile.Close() //nolint:errcheck

	memfdPath := fmt.Sprintf("/proc/self/fd/%d", memfdFile.Fd())

	// We have to open an O_NOFOLLOW|O_PATH to the memfd magic-link because we
	// cannot bind-mount the memfd itself (it's in the internal kernel mount
	// namespace and cross-mount-namespace bind-mounts are not allowed). This
	// also requires that this program stay alive continuously for the
	// magic-link to stay alive...
	memfdLink, err := os.OpenFile(memfdPath, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("mount: failed to /proc/self/fd magic-link for memfd: %w", err)
	}
	defer memfdLink.Close() //nolint:errcheck

	memfdLinkFdPath := fmt.Sprintf("/proc/self/fd/%d", memfdLink.Fd())

	exeFile, err := os.OpenFile(runcPath, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("mount: failed to open target runc binary path: %w", err)
	}
	defer exeFile.Close() //nolint:errcheck

	exeFdPath := fmt.Sprintf("/proc/self/fd/%d", exeFile.Fd())

	err = unix.Mount(memfdLinkFdPath, exeFdPath, "", unix.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("mount: failed to mount memfd on top of runc binary path target: %w", err)
	}

	// Clean up things we don't need...
	_ = exeFile.Close()   //nolint:errcheck
	_ = memfdLink.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return cleanup(runcPath, logger)
		case <-r.EventCh():
		}

		runtime.KeepAlive(memfdFile)
	}
}

// memfdClone is a memfd-only implementation of dmz.CloneBinary.
func memfdClone(path string) (*os.File, error) {
	binFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open runc binary path: %w", err)
	}
	defer binFile.Close() //nolint:errcheck

	stat, err := binFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("checking %s size: %w", path, err)
	}

	size := stat.Size()

	memfd, sealFn, err := dmz.Memfd("/proc/self/exe")
	if err != nil {
		return nil, fmt.Errorf("creating memfd failed: %w", err)
	}

	copied, err := io.Copy(memfd, binFile)
	if err != nil {
		return nil, fmt.Errorf("copy binary: %w", err)
	} else if copied != size {
		return nil, fmt.Errorf("copied binary size mismatch: %d != %d", copied, size)
	}

	if err := sealFn(&memfd); err != nil {
		return nil, fmt.Errorf("could not seal fd: %w", err)
	}

	if !dmz.IsCloned(memfd) {
		return nil, fmt.Errorf("cloned memfd is not properly sealed")
	}

	return memfd, nil
}

func cleanup(path string, logger *zap.Logger) error {
	file, err := os.OpenFile(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("cleanup: failed to open runc binary path: %w", err)
	}

	defer file.Close() //nolint:errcheck

	fdPath := fmt.Sprintf("/proc/self/fd/%d", file.Fd())

	// Keep umounting until we hit a umount error.
	for unix.Unmount(fdPath, unix.MNT_DETACH) == nil {
		// loop...
		logger.Info(fmt.Sprintf("memfd-bind: path %q unmount succeeded...", path))
	}

	logger.Info(fmt.Sprintf("memfd-bind: path %q has been cleared of all old bind-mounts", path))

	return nil
}
