// Package blockdevice provides a library for working with block devices.
package blockdevice

import (
	"fmt"
	"os"

	"github.com/autonomy/talos/src/initramfs/pkg/blockdevice/table"
	"github.com/autonomy/talos/src/initramfs/pkg/blockdevice/table/gpt"
	"golang.org/x/sys/unix"
)

// BlockDevice represents a block device.
type BlockDevice struct {
	table table.PartitionTable

	f *os.File
}

// Open initializes and returns a block device.
func Open(devname string) (*BlockDevice, error) {
	f, err := os.OpenFile(devname, os.O_RDWR, os.ModeDevice)
	if err != nil {
		return nil, err
	}
	// TODO: Dynamically detect MBR/GPT.
	// TODO: Use BLKGETSIZE ioctl to get the size.
	// TODO: Use BLKPBSZGET ioctl to get the physical sector size.
	// TODO: Use BLKSSZGET ioctl to get the logical sector size.
	// and pass them into gpt as options.
	bd := &BlockDevice{
		table: gpt.NewGPT(devname, f),
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
		return nil, fmt.Errorf("missing partition table")
	}

	return bd.table, nil
}

// RereadPartitionTable invokes the BLKRRPART ioctl have the kernel read the
// partition table.
func (bd *BlockDevice) RereadPartitionTable(devname string) error {
	f, err := os.Open(devname)
	if err != nil {
		return err
	}
	unix.Sync()
	if _, _, ret := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKRRPART, 0); ret != 0 {
		return fmt.Errorf("re-read partition table: %v", ret)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	unix.Sync()

	return nil
}
