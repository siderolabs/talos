/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package vfat

import (
	"os/exec"
)

// MakeFS creates a VFAT filesystem on the specified partition.
func MakeFS(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	args := []string{}

	if opts.Label != "" {
		args = append(args, "-F", "32", "-n", opts.Label)
	}

	args = append(args, partname)

	return cmd("mkfs.vfat", args...)
}

func cmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}
