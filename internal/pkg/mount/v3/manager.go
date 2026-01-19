// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
	"github.com/siderolabs/talos/pkg/xfs/opentree"
)

// Manager is the filesystem manager for mounting and unmounting filesystems.
type Manager struct {
	fs xfs.FS

	target  string
	printer func(string, ...any)

	selinuxLabel          string
	shared                bool
	skipIfMounted         bool
	keepOpen              bool
	detached              bool
	mountattr             int
	extraDirs             []string
	extraUnmountCallbacks []func(m *Manager)

	point *Point
}

// NewManager creates a new Manager with the given options.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{}

	for _, opt := range opts {
		opt.set(m)
	}

	return m
}

// Mount mounts a filesystem with the given options.
func (m *Manager) Mount() (*Point, error) {
	printer := discard
	if m.printer != nil {
		printer = m.printer
	}

	for _, dir := range m.extraDirs {
		printer("creating directory tree %q", dir)

		if !filepath.IsAbs(dir) {
			return nil, fmt.Errorf("dir %q is not absolute", dir)
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("error creating mount point directory %q: %w", dir, err)
		}
	}

	root := &xfs.UnixRoot{FS: m.fs}

	if err := root.OpenFS(); err != nil {
		return nil, fmt.Errorf("openfs failed: %w", err)
	}

	m.point = &Point{
		root:         root,
		detached:     m.detached,
		keepOpen:     m.keepOpen,
		target:       m.target,
		selinuxLabel: m.selinuxLabel,
	}

	opts := Options{
		Printer:         printer,
		Shared:          m.shared,
		SkipIfMounted:   m.skipIfMounted,
		MountAttributes: m.mountattr,
	}

	if !m.detached {
		if err := os.MkdirAll(m.target, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create mount target %s: %w", m.target, err)
		}

		printer("mounting %q to %q", m.point.Source(), m.target)
	}

	if err := m.point.Mount(opts); err != nil {
		return nil, fmt.Errorf("failed to mount: %w", err)
	}

	return m.point, nil
}

// Unmount the mount point.
func (m *Manager) Unmount() error {
	printer := discard
	if m.printer != nil {
		printer = m.printer
	}

	opts := UnmountOptions{
		Printer: printer,
	}

	for _, cb := range m.extraUnmountCallbacks {
		defer cb(m)
	}

	return m.point.Unmount(opts)
}

// Move the mount point to a new target.
func (m *Manager) Move(newTarget string) error {
	return m.point.Move(newTarget)
}

// ManagerOption is an option for configuring the Manager.
type ManagerOption struct {
	set func(*Manager)
}

// WithTarget sets the target mount point.
func WithTarget(target string) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.target = target
		},
	}
}

// WithKeepOpenAfterMount assesses if the mountpoint fd should be kept open after the Mount.
func WithKeepOpenAfterMount() ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.keepOpen = true
		},
	}
}

// WithSelinuxLabel sets the mount SELinux label.
func WithSelinuxLabel(label string) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.selinuxLabel = label
		},
	}
}

// WithShared sets the mount as shared.
func WithShared() ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.shared = true
		},
	}
}

// WithPrinter sets the printer function for logging.
func WithPrinter(printer func(string, ...any)) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.printer = printer
		},
	}
}

// WithSkipIfMounted sets the option to skip mounting if already mounted.
func WithSkipIfMounted() ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.skipIfMounted = true
		},
	}
}

// WithFsopen sets the filesystem opener with the given type and options.
func WithFsopen(fstype string, opts ...fsopen.Option) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.fs = fsopen.New(fstype, opts...)
		},
	}
}

// WithOpentreeFromPath sets the opentree opener with the path.
func WithOpentreeFromPath(path string) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.fs = opentree.NewFromPath(path)
		},
	}
}

// WithMountAttributes sets the mount attributes.
func WithMountAttributes(flags int) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.mountattr |= flags
		},
	}
}

// WithDisableAccessTime sets MOUNT_ATTR_NOATIME.
func WithDisableAccessTime() ManagerOption {
	return WithMountAttributes(unix.MOUNT_ATTR_NOATIME)
}

// WithSecure sets MOUNT_ATTR_NOSUID and MOUNT_ATTR_NODEV.
func WithSecure() ManagerOption {
	return WithMountAttributes(unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV)
}

// WithReadOnly sets the mount as read only.
func WithReadOnly() ManagerOption {
	return WithMountAttributes(unix.MOUNT_ATTR_RDONLY)
}

// WithDetached sets the mount as detached.
func WithDetached() ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.detached = true
			m.keepOpen = true
		},
	}
}

// WithExtraDirs adds extra dirs to the manager that should be created prior to mounting the filesystem.
func WithExtraDirs(dirs ...string) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.extraDirs = append(m.extraDirs, dirs...)
		},
	}
}

// WithExtraUnmountCallbacks adds extra callbacks to the unmount operation.
// Those need to handle errors by themselves.
func WithExtraUnmountCallbacks(callbacks ...func(m *Manager)) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.extraUnmountCallbacks = append(m.extraUnmountCallbacks, callbacks...)
		},
	}
}
