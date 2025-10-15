// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadCgroupfsProperty reads a property from cgroupfs into an existing Node.
func ReadCgroupfsProperty(node *Node, cgroupPath, property string) error {
	f, err := os.OpenFile(filepath.Join(cgroupPath, property), os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("error opening cgroupfs file %w", err)
	}

	defer f.Close() //nolint:errcheck

	err = node.Parse(property, f)
	if err != nil {
		return fmt.Errorf("error parsing cgroupfs file %w", err)
	}

	return nil
}

// GetCgroupProperty reads a property from cgroupfs into a new Node.
func GetCgroupProperty(cgroupPath, property string) (*Node, error) {
	node := Node{}
	err := ReadCgroupfsProperty(&node, cgroupPath, property)

	return &node, err
}
