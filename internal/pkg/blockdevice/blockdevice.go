/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package blockdevice provides a library for working with block devices.
package blockdevice

import (
	"bytes"
	"fmt"
	"os"

	"github.com/autonomy/talos/internal/pkg/blockdevice/table"
	"github.com/autonomy/talos/internal/pkg/blockdevice/table/gpt"
	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

// BlockDevice represents a block device.
type BlockDevice struct {
	table table.PartitionTable

	f *os.File
}

// Open initializes and returns a block device.
// TODO(andrewrynhard): Use BLKGETSIZE ioctl to get the size.
// TODO(andrewrynhard): Use BLKPBSZGET ioctl to get the physical sector size.
// TODO(andrewrynhard): Use BLKSSZGET ioctl to get the logical sector size
// and pass them into gpt as options.
func Open(devname string, setters ...Option) (*BlockDevice, error) {
	opts := NewDefaultOptions(setters...)

	bd := &BlockDevice{}

	f, err := os.OpenFile(devname, os.O_RDWR, os.ModeDevice)
	if err != nil {
		return nil, err
	}

	bd.f = f

	if opts.CreateGPT {
		gpt := gpt.NewGPT(devname, f)
		table, e := gpt.New()
		if e != nil {
			return nil, e
		}
		bd.table = table
	} else {
		buf := make([]byte, 1)
		// PMBR protective entry starts at 446. The partition type is at offset
		// 4 from the start of the PMBR protective entry.
		_, err = f.ReadAt(buf, 450)
		if err != nil {
			return nil, err
		}
		// For GPT, the partition type should be 0xee (EFI GPT).
		if bytes.Equal(buf, []byte{0xee}) {
			bd.table = gpt.NewGPT(devname, f)
		} else {
			return nil, errors.New("failed to find GUID partition table")
		}
	}

	return bd, nil
}

// Close closes the block devices's open file.
func (bd *BlockDevice) Close() error {
	return bd.f.Close()
}

// PartitionTable returns the block device partition table.
func (bd *BlockDevice) PartitionTable(read bool) (table.PartitionTable, error) {
	if bd.table == nil {
		return nil, fmt.Errorf("missing partition table")
	}

	if !read {
		return bd.table, nil
	}

	return bd.table, bd.table.Read()
}

// RereadPartitionTable invokes the BLKRRPART ioctl to have the kernel read the
// partition table.
func (bd *BlockDevice) RereadPartitionTable() error {
	unix.Sync()
	if _, _, ret := unix.Syscall(unix.SYS_IOCTL, bd.f.Fd(), unix.BLKRRPART, 0); ret != 0 {
		return fmt.Errorf("re-read partition table: %v", ret)
	}
	if err := bd.f.Sync(); err != nil {
		return err
	}
	unix.Sync()

	return nil
}
