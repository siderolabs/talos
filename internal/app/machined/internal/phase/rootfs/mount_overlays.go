/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"

	"golang.org/x/sys/unix"
)

const (
	// SystemVarPath is the path to write runtime system related files and
	// directories.
	SystemVarPath = "/var/system"
)

// MountOverlay represents the MountOverlay task.
type MountOverlay struct{}

// NewMountOverlayTask initializes and returns an MountOverlay task.
func NewMountOverlayTask() phase.Task {
	return &MountOverlay{}
}

// RuntimeFunc returns the runtime function.
func (task *MountOverlay) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return task.container
	default:
		return task.standard
	}
}

func (task *MountOverlay) standard(platform platform.Platform, data *userdata.UserData) (err error) {
	overlays := []string{
		"/etc/kubernetes",
		"/etc/cni",
		"/usr/libexec/kubernetes",
		"/usr/etc/udev",
		"/opt",
	}
	// Create all overlay mounts.
	for _, o := range overlays {
		if err = overlay(o); err != nil {
			return err
		}
	}

	return nil
}

func (task *MountOverlay) container(platform platform.Platform, data *userdata.UserData) (err error) {
	targets := []string{"/", "/var/lib/kubelet", "/etc/cni"}
	for _, t := range targets {
		if err = unix.Mount("", t, "", unix.MS_SHARED, ""); err != nil {
			return err
		}
	}

	return nil
}

func overlay(path string) error {
	parts := strings.Split(path, "/")
	prefix := strings.Join(parts[1:], "-")
	diff := fmt.Sprintf(filepath.Join(SystemVarPath, "%s-diff"), prefix)
	workdir := fmt.Sprintf(filepath.Join(SystemVarPath, "%s-workdir"), prefix)

	for _, s := range []string{diff, workdir} {
		if err := os.MkdirAll(s, 0700); err != nil {
			return err
		}
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", path, diff, workdir)
	if err := unix.Mount("overlay", path, "overlay", 0, opts); err != nil {
		return errors.Errorf("error creating overlay mount to %s: %v", path, err)
	}

	return nil
}
