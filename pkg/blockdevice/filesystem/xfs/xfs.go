/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package xfs provides an interface to xfsprogs.
package xfs

import (
	"os/exec"
)

// GrowFS expands a XFS filesystem to the maximum possible. The partition
// MUST be mounted, or this will fail.
func GrowFS(partname string) error {
	return cmd("xfs_growfs", "-d", partname)
}

// MakeFS creates a XFS filesystem on the specified partition.
func MakeFS(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	// The ftype=1 naming option is required by overlayfs.
	args := []string{"-n", "ftype=1"}

	if opts.Force {
		args = append(args, "-f")
	}

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	args = append(args, partname)

	return cmd("mkfs.xfs", args...)
}

func cmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}
