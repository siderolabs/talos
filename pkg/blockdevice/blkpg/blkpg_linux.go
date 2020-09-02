// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blkpg

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/blockdevice/lba"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
)

// InformKernelOfAdd invokes the BLKPG_ADD_PARTITION ioctl.
func InformKernelOfAdd(f *os.File, partition table.Partition) error {
	return inform(f, partition, unix.BLKPG_ADD_PARTITION)
}

// InformKernelOfResize invokes the BLKPG_RESIZE_PARTITION ioctl.
func InformKernelOfResize(f *os.File, partition table.Partition) error {
	return inform(f, partition, unix.BLKPG_RESIZE_PARTITION)
}

// InformKernelOfDelete invokes the BLKPG_DEL_PARTITION ioctl.
func InformKernelOfDelete(f *os.File, partition table.Partition) error {
	return inform(f, partition, unix.BLKPG_DEL_PARTITION)
}

func inform(f *os.File, partition table.Partition, op int32) (err error) {
	var (
		start  int64
		length int64
	)

	switch op {
	case unix.BLKPG_DEL_PARTITION:
		start = 0
		length = 0
	default:
		var l *lba.LogicalBlockAddresser

		if l, err = lba.New(f); err != nil {
			return err
		}

		blocksize := int64(l.LogicalBlockSize)

		start = partition.Start() * blocksize
		length = partition.Length() * blocksize
	}

	data := &unix.BlkpgPartition{
		Start:  start,
		Length: length,
		Pno:    partition.No(),
	}

	arg := &unix.BlkpgIoctlArg{
		Op:      op,
		Datalen: int32(unsafe.Sizeof(*data)),
		Data:    (*byte)(unsafe.Pointer(data)),
	}

	err = retry.Constant(10*time.Second, retry.WithUnits(500*time.Millisecond)).Retry(func() error {
		_, _, errno := syscall.Syscall(
			syscall.SYS_IOCTL,
			f.Fd(),
			unix.BLKPG,
			uintptr(unsafe.Pointer(arg)),
		)

		if errno != 0 {
			switch errno { //nolint: exhaustive
			case unix.EBUSY:
				return retry.ExpectedError(err)
			default:
				return retry.UnexpectedError(err)
			}
		}

		if err = f.Sync(); err != nil {
			return retry.UnexpectedError(err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to inform kernel: %w", err)
	}

	return nil
}
