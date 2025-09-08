// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package xfs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// OSRoot represents a filesystem wrapper using os interface.
type OSRoot struct {
	Shadow string
}

// Interface guard.
var _ interface {
	Root
} = (*OSRoot)(nil)

// OpenFS is no-op for OSRoot.
func (root *OSRoot) OpenFS() error {
	return nil
}

// Close is no-op for OSRoot.
func (root *OSRoot) Close() error {
	return nil
}

// RepairFS is no-op for OSRoot.
func (root *OSRoot) RepairFS() error {
	return nil
}

// Fd is no-op for OSRoot.
func (root *OSRoot) Fd() (int, error) {
	return 0, os.ErrInvalid
}

// Mkdir creates a new directory in the root filesystem with the specified name and permissions.
func (root *OSRoot) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(filepath.Join(root.Shadow, name), perm)
}

// Open opens a file in the root filesystem with the specified name in read-only mode.
func (root *OSRoot) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(root.Shadow, name))
}

// OpenFile opens a file in the root filesystem with the specified name, flags, and permissions.
func (root *OSRoot) OpenFile(name string, flags int, perm os.FileMode) (File, error) {
	return os.OpenFile(filepath.Join(root.Shadow, name), flags, perm)
}

// Remove removes a file or directory from the root filesystem.
func (root *OSRoot) Remove(name string) error {
	return os.Remove(filepath.Join(root.Shadow, name))
}

// Source returns the source of the underlying filesystem.
func (root *OSRoot) Source() string {
	return root.Shadow
}

// FSType returns the type of the underlying filesystem.
func (root *OSRoot) FSType() string {
	return "os"
}
