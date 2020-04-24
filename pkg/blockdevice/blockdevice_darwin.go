// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package blockdevice provides a library for working with block devices.
package blockdevice

import (
	"fmt"
	"os"

	"github.com/talos-systems/talos/pkg/blockdevice/table"
)

// BlockDevice represents a block device.
type BlockDevice struct {
	table table.PartitionTable

	f *os.File
}

// Open initializes and returns a block device.
// TODO(andrewrynhard): Use BLKGETSIZE ioctl to get the size.
func Open(devname string, setters ...Option) (bd *BlockDevice, err error) {
	return nil, fmt.Errorf("not implemented")
}

// Close closes the block devices's open file.
func (bd *BlockDevice) Close() error {
	return fmt.Errorf("not implemented")
}

// PartitionTable returns the block device partition table.
func (bd *BlockDevice) PartitionTable(read bool) (table.PartitionTable, error) {
	return nil, fmt.Errorf("not implemented")
}

// RereadPartitionTable invokes the BLKRRPART ioctl to have the kernel read the
// partition table.
//
// NB: Rereading the partition table requires that all partitions be
// unmounted or it will fail with EBUSY.
func (bd *BlockDevice) RereadPartitionTable() error {
	return fmt.Errorf("not implemented")
}

// Device returns the backing file for the block device.
func (bd *BlockDevice) Device() *os.File {
	return nil
}

// Size returns the size of the block device in bytes.
func (bd *BlockDevice) Size() (uint64, error) {
	return 0, fmt.Errorf("not implemented")
}

// Reset will reset a block device given a device name.
// Simply deletes partition table on device.
func (bd *BlockDevice) Reset() error {
	return fmt.Errorf("not implemented")
}
