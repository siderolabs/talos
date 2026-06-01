// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/freddierice/go-losetup/v2"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

//
// NOTE: This file is a rewrite of various helpers from mount/v2.
//

func discard(string, ...any) {}

// NewCgroup2 creates a new cgroup2 filesystem.
func NewCgroup2() *Manager {
	return NewManager(
		WithTarget(constants.CgroupMountPath),
		WithSecure(),
		WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
		WithFsopen(
			"cgroup2",
			fsopen.WithBoolParameter("nsdelegate"),
			fsopen.WithBoolParameter("memory_recursiveprot"),
		),
	)
}

// NewReadOnlyOverlay creates a new read-only overlay filesystem.
func NewReadOnlyOverlay(sources []string, target string, printer func(string, ...any), options ...ManagerOption) *Manager {
	fsOptions := []fsopen.Option{}

	if printer != nil {
		printer("mounting %d overlays: %v", len(sources), sources)
	}

	if len(sources) > 1 {
		for _, source := range sources {
			fsOptions = append(fsOptions, fsopen.WithStringParameter("lowerdir+", source))
		}
	} else if len(sources) == 1 {
		fsOptions = append(fsOptions, fsopen.WithStringParameter("lowerdir", sources[0]))
	}

	options = append(
		options,
		WithPrinter(printer),
		WithTarget(target),
		WithReadOnly(),
		WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NODEV),
		WithFsopen("overlay", fsOptions...),
	)

	return NewManager(options...)
}

// NewOverlayWithBasePath creates a new overlay filesystem with a base path.
func NewOverlayWithBasePath(sources []string, target, basePath string, printer func(string, ...any), options ...ManagerOption) *Manager {
	_, overlayPrefix, _ := strings.Cut(target, "/")
	overlayPrefix = strings.ReplaceAll(overlayPrefix, "/", "-")

	diff := fmt.Sprintf(filepath.Join(basePath, "%s-diff"), overlayPrefix)
	workdir := fmt.Sprintf(filepath.Join(basePath, "%s-workdir"), overlayPrefix)

	fsOptions := []fsopen.Option{
		fsopen.WithStringParameter("upperdir", diff),
		fsopen.WithStringParameter("workdir", workdir),
	}

	if len(sources) > 1 {
		for _, source := range sources {
			fsOptions = append(fsOptions, fsopen.WithStringParameter("lowerdir+", source))
		}
	} else if len(sources) == 1 {
		fsOptions = append(fsOptions, fsopen.WithStringParameter("lowerdir", sources[0]))
	}

	options = append(
		options,
		WithTarget(target),
		WithExtraDirs(diff, workdir),
		WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NODEV),
		WithFsopen("overlay", fsOptions...),
		WithPrinter(printer),
	)

	return NewManager(options...)
}

// NewVarOverlay creates a new /var overlay filesystem.
func NewVarOverlay(sources []string, target string, printer func(string, ...any), options ...ManagerOption) *Manager {
	return NewOverlayWithBasePath(sources, target, constants.VarSystemOverlaysPath, printer, options...)
}

// NewSystemOverlay creates a new /system overlay filesystem.
func NewSystemOverlay(sources []string, target string, printer func(string, ...any), options ...ManagerOption) *Manager {
	return NewOverlayWithBasePath(sources, target, constants.SystemOverlaysPath, printer, options...)
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
		WithReadOnly(),
		WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NODEV),
		WithShared(),
		WithExtraUnmountCallbacks(func(m *Manager) {
			dev.Detach() //nolint:errcheck
		}),
		WithFsopen(
			"squashfs",
			fsopen.WithSource(dev.Path()),
			fsopen.WithBoolParameter("ro"),
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

	if err == nil || errors.Is(err, os.ErrNotExist) {
		return err == nil
	}

	// this means something else is wrong, let's panic
	// as this should never happen
	panic(err)
}

// Pseudo creates basic filesystem mountpoint managers.
func Pseudo(printer func(string, ...any)) Managers {
	return gather(
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/dev"),
			WithKeepOpenAfterMount(),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID),
			WithFsopen(
				"devtmpfs",
				fsopen.WithStringParameter("mode", "0755"),
			),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/proc"),
			WithKeepOpenAfterMount(),
			WithSecure(),
			WithFsopen("proc"),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/sys"),
			WithKeepOpenAfterMount(),
			WithSecure(),
			WithFsopen("sysfs"),
		),
	)
}

// PseudoLate creates late pseudo filesystem mountpoint managers.
func PseudoLate(printer func(string, ...any)) Managers {
	return gather(
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/run"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithSelinuxLabel(constants.RunSelinuxLabel),
			WithRecursiveUnmount(),
			WithFsopen(
				"tmpfs",
				fsopen.WithStringParameter("mode", "0755"),
			),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/system"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithSelinuxLabel(constants.SystemSelinuxLabel),
			WithRecursiveUnmount(),
			WithFsopen(
				"tmpfs",
				fsopen.WithStringParameter("mode", "0755"),
			),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/tmp"),
			WithSecure(),
			WithFsopen(
				"tmpfs",
				fsopen.WithStringParameter("mode", "0755"),
				fsopen.WithStringParameter("size", "64M"),
			),
		),
	)
}

// PseudoSub creates additional pseudo filesystem mountpoint managers.
func PseudoSub(printer func(string, ...any)) Managers {
	return gather(
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/dev/shm"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithFsopen("tmpfs"),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/dev/pts"),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NOEXEC),
			WithFsopen(
				"devpts",
				fsopen.WithStringParameter("ptmxmode", "000"),
				fsopen.WithStringParameter("mode", "620"),
				fsopen.WithStringParameter("gid", "5"),
			),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithMountAttributes(unix.MOUNT_ATTR_NOSUID|unix.MOUNT_ATTR_NODEV),
			WithTarget("/dev/hugepages"),
			WithFsopen("hugetlbfs"),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/sys/fs/bpf"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithFsopen("bpf"),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/sys/kernel/security"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithFsopen("securityfs"),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/sys/kernel/tracing"),
			WithSecure(),
			WithFsopen("tracefs"),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/sys/kernel/config"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithFsopen("configfs"),
		),
		newManager(
			always,
			WithPrinter(printer),
			WithTarget("/sys/kernel/debug"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithFsopen("debugfs"),
		),
		newManager(
			selinux.IsEnabled,
			WithPrinter(printer),
			WithTarget("/sys/fs/selinux"),
			WithSecure(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithFsopen("selinuxfs"),
		),
		newManager(
			hasEFIVars,
			WithPrinter(printer),
			WithTarget(constants.EFIVarsMountPoint),
			WithSecure(),
			WithReadOnly(),
			WithMountAttributes(unix.MOUNT_ATTR_RELATIME),
			WithFsopen("efivarfs"),
		),
	)
}
