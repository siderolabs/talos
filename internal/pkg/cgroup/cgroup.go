// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cgroup provides cgroup utilities to handle nested cgroups.
//
// When Talos runs in a container, it might either share or not share the host cgroup namespace.
// If the cgroup namespace is not shared, PID 1 will appear in cgroup '/', otherwise it will be
// part of some pre-existing cgroup hierarchy.
//
// When Talos is running in a non-container mode, it is always at the root of the cgroup hierarchy.
//
// This package provides a transparent way to handle nested cgroups by providing a Path() function
// which returns the correct cgroup path based on the cgroup hierarchy available.
package cgroup

import (
	"path/filepath"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup2"
)

var root = "/"

// InitRoot initializes the root cgroup path.
//
// This function should be called once at the beginning of the program, after the cgroup
// filesystem is mounted.
//
// This function only supports cgroupv2 nesting.
func InitRoot() error {
	if cgroups.Mode() != cgroups.Unified {
		return nil
	}

	var err error

	root, err = cgroup2.NestedGroupPath("/")

	return err
}

// Root returns the root cgroup path.
func Root() string {
	return root
}

// Path returns the path to the cgroup.
//
// This function handles the case when the cgroups are nested.
func Path(cgroupPath string) string {
	if cgroups.Mode() != cgroups.Unified {
		return cgroupPath
	}

	return filepath.Join(root, cgroupPath)
}
