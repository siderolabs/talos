// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

// Package opentree provides a simple interface to create and manage a subfilesystem
// using the `open_tree` syscall. It allows for creating a new subfilesystem
// by cloning an existing filesystem tree and provides a method to close the filesystem
// when it is no longer needed.
package opentree
