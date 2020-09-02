// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blockdevice

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/table/gpt"
)

// BlockDevice represents a block device.
type BlockDevice struct {
	table table.PartitionTable

	f *os.File
}

// Open initializes and returns a block device.
// TODO(andrewrynhard): Use BLKGETSIZE ioctl to get the size.
func Open(devname string, setters ...Option) (bd *BlockDevice, err error) {
	opts := NewDefaultOptions(setters...)

	bd = &BlockDevice{}

	var f *os.File

	if f, err = os.OpenFile(devname, os.O_RDWR|unix.O_CLOEXEC, os.ModeDevice); err != nil {
		return nil, err
	}

	bd.f = f

	defer func() {
		if err != nil {
			// nolint: errcheck
			f.Close()
		}
	}()

	if opts.CreateGPT {
		var g *gpt.GPT

		if g, err = gpt.NewGPT(devname, f); err != nil {
			return nil, err
		}

		var pt table.PartitionTable

		if pt, err = g.New(); err != nil {
			return nil, err
		}

		if err = pt.Write(); err != nil {
			return nil, err
		}

		bd.table = pt
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
			var g *gpt.GPT
			if g, err = gpt.NewGPT(devname, f); err != nil {
				return nil, err
			}
			bd.table = g
		}
	}

	return bd, nil
}

// Close closes the block devices's open file.
func (bd *BlockDevice) Close() error {
	return bd.f.Close()
}

// PartitionTable returns the block device partition table.
func (bd *BlockDevice) PartitionTable() (table.PartitionTable, error) {
	if bd.table == nil {
		return nil, ErrMissingPartitionTable
	}

	return bd.table, bd.table.Read()
}

// RereadPartitionTable invokes the BLKRRPART ioctl to have the kernel read the
// partition table.
//
// NB: Rereading the partition table requires that all partitions be
// unmounted or it will fail with EBUSY.
func (bd *BlockDevice) RereadPartitionTable() error {
	// Flush the file buffers.
	// NOTE(andrewrynhard): I'm not entirely sure we need this, but
	// figured it wouldn't hurt.
	if err := bd.f.Sync(); err != nil {
		return err
	}
	// Flush the block device buffers.
	if _, _, ret := unix.Syscall(unix.SYS_IOCTL, bd.f.Fd(), unix.BLKFLSBUF, 0); ret != 0 {
		return fmt.Errorf("flush block device buffers: %v", ret)
	}

	var (
		err error
		ret syscall.Errno
	)

	// Reread the partition table.
	err = retry.Constant(5*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
		if _, _, ret = unix.Syscall(unix.SYS_IOCTL, bd.f.Fd(), unix.BLKRRPART, 0); ret == 0 {
			return nil
		}
		switch ret { //nolint: exhaustive
		case syscall.EBUSY:
			return retry.ExpectedError(err)
		default:
			return retry.UnexpectedError(err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to re-read partition table: %w", err)
	}

	return err
}

// Device returns the backing file for the block device.
func (bd *BlockDevice) Device() *os.File {
	return bd.f
}

// Size returns the size of the block device in bytes.
func (bd *BlockDevice) Size() (uint64, error) {
	var devsize uint64
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, bd.f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&devsize))); errno != 0 {
		return 0, errno
	}

	return devsize, nil
}

// Reset will reset a block device given a device name.
// Simply deletes partition table on device.
func (bd *BlockDevice) Reset() (err error) {
	var pt table.PartitionTable

	if pt, err = bd.PartitionTable(); err != nil {
		return err
	}

	for _, p := range pt.Partitions() {
		if err = pt.Delete(p); err != nil {
			return fmt.Errorf("failed to delete partition: %w", err)
		}
	}

	if err = pt.Write(); err != nil {
		return err
	}

	return nil
}
