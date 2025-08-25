// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/freddierice/go-losetup/v2"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/internal/pkg/xfs/fsopen"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//
// NOTE: This file is a rewrite of various helpers from mount/v2.
//

// NewCgroup2 creates a new cgroup2 filesystem.
func NewCgroup2() *Manager {
	return NewManager(
		WithTarget(constants.CgroupMountPath),
		WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NODEV|unix.MOUNT_ATTR_NOEXEC|unix.MOUNT_ATTR_RELATIME),
		WithFsopen(
			"cgroup2",
			fsopen.WithBoolParameter("nsdelegate"),
			fsopen.WithBoolParameter("memory_recursiveprot"),
		),
	)
}

// NewReadOnlyOverlay creates a new read-only overlay filesystem.
func NewReadOnlyOverlay(sources []string, target string, options ...ManagerOption) *Manager {
	fsOptions := []fsopen.Option{
		fsopen.WithStringParameter("lowerdir", strings.Join(sources, ":")),
	}

	options = append(options,
		WithTarget(target),
		WithMountAttributes(unix.MOUNT_ATTR_RDONLY),
		WithFsopen("overlay", fsOptions...),
	)

	return NewManager(options...)
}

// NewOverlayWithBasePath creates a new overlay filesystem with a base path.
func NewOverlayWithBasePath(sources []string, target, basePath string, options ...ManagerOption) *Manager {
	_, overlayPrefix, _ := strings.Cut(target, "/")
	overlayPrefix = strings.ReplaceAll(overlayPrefix, "/", "-")

	diff := fmt.Sprintf(filepath.Join(basePath, "%s-diff"), overlayPrefix)
	workdir := fmt.Sprintf(filepath.Join(basePath, "%s-workdir"), overlayPrefix)

	fsOptions := []fsopen.Option{
		fsopen.WithStringParameter("lowerdir", strings.Join(sources, ":")),
		fsopen.WithStringParameter("upperdir", diff),
		fsopen.WithStringParameter("workdir", workdir),
	}

	options = append(options,
		WithTarget(target),
		WithFsopen("overlay", fsOptions...),
	)

	return NewManager(options...)
}

// NewVarOverlay creates a new /var overlay filesystem.
func NewVarOverlay(sources []string, target string, options ...ManagerOption) *Manager {
	return NewOverlayWithBasePath(sources, target, constants.VarSystemOverlaysPath, options...)
}

// NewSystemOverlay creates a new /system overlay filesystem.
func NewSystemOverlay(sources []string, target string, options ...ManagerOption) *Manager {
	return NewOverlayWithBasePath(sources, target, constants.SystemOverlaysPath, options...)
}

// Squashfs binds the squashfs to the loop device and returns the mountpoint for it to the specified target.
func Squashfs(target, squashfsFile string, printer func(string, ...any)) (*Manager, error) {
	dev, err := losetup.Attach(squashfsFile, 0, true)
	if err != nil {
		return nil, err
	}

	return NewManager(
		WithTarget(target),
		WithPrinter(printer),
		WithMountAttributes(unix.MOUNT_ATTR_RDONLY),
		WithFsopen(
			"squashfs",
			fsopen.WithSource(dev.Path()),
			fsopen.WithBoolParameter("ro"),
			fsopen.WithPrinter(printer),
		),
	), nil
}

func gather[T comparable](c ...func() T) []T {
	var (
		zero T
		vals []T
	)

	for _, f := range c {
		val := f()

		if val != zero {
			vals = append(vals, val)
		}
	}

	return vals
}

func newManager(condition func() bool, opts ...ManagerOption) func() *Manager {
	return func() *Manager {
		if !condition() {
			return nil
		}

		return NewManager(opts...)
	}
}

func always() bool { return true }

func hasEFIVars() bool {
	_, err := os.Stat(constants.EFIVarsMountPoint)

	return err == nil
}

// Pseudo creates basic filesystem mountpoint managers.
func Pseudo(printer func(string, ...any)) Managers {
	return gather(
		newManager(
			always,
			WithTarget("/dev"),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID),
			WithFsopen(
				"devtmpfs",
				fsopen.WithStringParameter("mode", "0755"),
				fsopen.WithPrinter(printer),
			),
		),
		newManager(
			always,
			WithTarget("/proc"),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NOEXEC|unix.MOUNT_ATTR_NODEV),
			WithFsopen("proc", fsopen.WithPrinter(printer))),
		newManager(
			always,
			WithTarget("/sys"),
			WithFsopen("sysfs", fsopen.WithPrinter(printer)),
		),
	)
}

// PseudoLate creates late pseudo filesystem mountpoint managers.
func PseudoLate(printer func(string, ...any)) Managers {
	return gather(
		newManager(always, WithTarget("/run"),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NOEXEC|unix.MOUNT_ATTR_RELATIME),
			WithFsopen(
				"tmpfs",
				fsopen.WithPrinter(printer),
				fsopen.WithStringParameter("mode", "0755"),
			),
		),
		newManager(always, WithTarget("/system"),
			WithFsopen(
				"tmpfs",
				fsopen.WithPrinter(printer),
				fsopen.WithStringParameter("mode", "0755"),
			),
		),
		newManager(always, WithTarget("/tmp"),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NOEXEC|unix.MOUNT_ATTR_NODEV),
			WithFsopen(
				"tmpfs",
				fsopen.WithPrinter(printer),
				fsopen.WithStringParameter("mode", "0755"),
				fsopen.WithStringParameter("size", "64M"),
			),
		),
	)
}

// PseudoSub creates additional pseudo filesystem mountpoint managers.
func PseudoSub(printer func(string, ...any)) Managers {
	return gather(
		newManager(always, WithTarget("/dev/shm"), WithFsopen("tmpfs", fsopen.WithPrinter(printer))),
		newManager(
			always,
			WithTarget("/dev/pts"),
			WithFsopen(
				"devpts",
				fsopen.WithStringParameter("ptmxmode", "000"),
				fsopen.WithStringParameter("mode", "620"),
				fsopen.WithStringParameter("gid", "5"),
				fsopen.WithPrinter(printer),
			),
		),
		newManager(always, WithTarget("/dev/hugepages"), WithFsopen("hugetlbfs", fsopen.WithPrinter(printer))),
		newManager(always, WithTarget("/sys/fs/bpf"), WithFsopen("bpf", fsopen.WithPrinter(printer))),
		newManager(always, WithTarget("/sys/kernel/security"), WithFsopen("securityfs", fsopen.WithPrinter(printer))),
		newManager(always, WithTarget("/sys/kernel/tracing"), WithFsopen("tracefs", fsopen.WithPrinter(printer))),
		newManager(always, WithTarget("/sys/kernel/config"), WithFsopen("configfs", fsopen.WithPrinter(printer))),
		newManager(always, WithTarget("/sys/kernel/debug"), WithFsopen("debugfs", fsopen.WithPrinter(printer))),
		newManager(
			selinux.IsEnabled,
			WithTarget("/sys/fs/selinux"),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NOEXEC|unix.MOUNT_ATTR_RELATIME),
			WithFsopen(
				"selinuxfs",
				fsopen.WithPrinter(printer),
			),
		),
		newManager(
			hasEFIVars,
			WithTarget(constants.EFIVarsMountPoint),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NOEXEC|unix.MOUNT_ATTR_NODEV|unix.MOUNT_ATTR_RELATIME|unix.MOUNT_ATTR_RDONLY),
			WithFsopen(
				"efivarfs",
				fsopen.WithPrinter(printer),
			),
		),
	)
}
