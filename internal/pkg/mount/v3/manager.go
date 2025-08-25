// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"fmt"
	"os"

	"github.com/siderolabs/talos/internal/pkg/xfs"
	"github.com/siderolabs/talos/internal/pkg/xfs/fsopen"
)

// Manager is the filesystem manager for mounting and unmounting filesystems.
type Manager struct {
	fs xfs.FS

	target  string
	printer func(string, ...any)

	selinuxLabel  string
	shared        bool
	skipIfMounted bool
	mountattr     int

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
	if m.point == nil {
		root := &xfs.UnixRoot{FS: m.fs}

		if err := root.OpenFS(); err != nil {
			return nil, fmt.Errorf("openfs failed: %w", err)
		}

		m.point = &Point{
			root:         root,
			target:       m.target,
			selinuxLabel: m.selinuxLabel,
		}
	}

	opts := Options{
		Printer:         m.printer,
		Shared:          m.shared,
		SkipIfMounted:   m.skipIfMounted,
		MountAttributes: m.mountattr,
	}

	if err := os.MkdirAll(m.target, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create mount target %s: %w", m.target, err)
	}

	if err := m.point.Mount(opts); err != nil {
		return nil, fmt.Errorf("failed to mount: %w", err)
	}

	return m.point, nil
}

// Unmount the mount point.
func (m *Manager) Unmount() error {
	opts := UnmountOptions{
		Printer: m.printer,
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

// WithMountAttributes sets the mount attributes.
func WithMountAttributes(flags int) ManagerOption {
	return ManagerOption{
		set: func(m *Manager) {
			m.mountattr |= flags
		},
	}
}
