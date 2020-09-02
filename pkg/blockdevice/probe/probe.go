// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package probe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/iso9660"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/xfs"
	gptpartition "github.com/talos-systems/talos/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
)

// ProbedBlockDevice represents a probed block device.
type ProbedBlockDevice struct {
	*blockdevice.BlockDevice

	SuperBlock filesystem.SuperBlocker
	Path       string
}

// All probes a block device's file system for the given label.
func All() (all []*ProbedBlockDevice, err error) {
	var infos []os.FileInfo

	if infos, err = ioutil.ReadDir("/sys/block"); err != nil {
		return nil, err
	}

	for _, info := range infos {
		devpath := "/dev/" + info.Name()

		var probed []*ProbedBlockDevice

		probed, err = probeFilesystem(devpath)
		if err != nil {
			return nil, err
		}

		all = append(all, probed...)
	}

	return all, nil
}

// FileSystem probes the provided path's file system.
func FileSystem(path string) (sb filesystem.SuperBlocker, err error) {
	var f *os.File
	// Sleep for up to 5s to wait for kernel to create the necessary device files.
	// If we dont sleep this becomes racy in that the device file does not exist
	// and it will fail to open.
	err = retry.Constant(5*time.Second, retry.WithUnits((50 * time.Millisecond))).Retry(func() error {
		if f, err = os.OpenFile(path, os.O_RDONLY|unix.O_CLOEXEC, os.ModeDevice); err != nil {
			if os.IsNotExist(err) {
				return retry.ExpectedError(err)
			}
			return retry.UnexpectedError(err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// nolint: errcheck
	defer f.Close()

	superblocks := []filesystem.SuperBlocker{
		&iso9660.SuperBlock{},
		&vfat.SuperBlock{},
		&xfs.SuperBlock{},
	}

	for _, sb := range superblocks {
		if _, err = f.Seek(sb.Offset(), io.SeekStart); err != nil {
			return nil, err
		}

		err = binary.Read(f, binary.BigEndian, sb)
		if err != nil {
			return nil, err
		}

		if sb.Is() {
			return sb, nil
		}
	}

	return nil, nil
}

// GetDevWithFileSystemLabel probes all known block device's file systems for
// the given label.
func GetDevWithFileSystemLabel(value string) (probe *ProbedBlockDevice, err error) {
	var probed []*ProbedBlockDevice

	if probed, err = All(); err != nil {
		return nil, err
	}

	return filterByLabel(probed, value)
}

// DevForFileSystemLabel probes a block device's file systems for the
// given label.
func DevForFileSystemLabel(devpath, value string) (probe *ProbedBlockDevice, err error) {
	var probed []*ProbedBlockDevice

	probed, err = probeFilesystem(devpath)
	if err != nil {
		return nil, err
	}

	probe, err = filterByLabel(probed, value)
	if err != nil {
		return nil, err
	}

	return probe, err
}

func probe(devpath string) (devpaths []string) {
	devpaths = []string{}

	// Start by opening the block device.
	// If a partition table was not found, it is still possible that a
	// file system exists without a partition table.
	bd, err := blockdevice.Open(devpath)
	if err != nil {
		// nolint: errcheck
		if sb, _ := FileSystem(devpath); sb != nil {
			devpaths = append(devpaths, devpath)
		}

		return devpaths
	}
	// nolint: errcheck
	defer bd.Close()

	// A partition table was not found, and we have already checked for
	// a file system on the block device. Let's check if the block device
	// has partitions.
	pt, err := bd.PartitionTable()
	if err != nil {
		return devpaths
	}

	// A partition table was found, now probe each partition's file system.
	name := filepath.Base(devpath)

	for _, p := range pt.Partitions() {
		partpath, err := util.PartPath(name, int(p.No()))
		if err != nil {
			return devpaths
		}

		// nolint: errcheck
		if sb, _ := FileSystem(partpath); sb != nil {
			devpaths = append(devpaths, partpath)
		}
	}

	return devpaths
}

// GetBlockDeviceWithPartitonName probes all known block device's partition
// table for a parition with the specified name.
func GetBlockDeviceWithPartitonName(name string) (bd *blockdevice.BlockDevice, err error) {
	var infos []os.FileInfo

	if infos, err = ioutil.ReadDir("/sys/block"); err != nil {
		return nil, err
	}

	for _, info := range infos {
		devpath := "/dev/" + info.Name()

		if bd, err = blockdevice.Open(devpath); err != nil {
			continue
		}

		pt, err := bd.PartitionTable()
		if err != nil {
			// nolint: errcheck
			bd.Close()

			if errors.Is(err, blockdevice.ErrMissingPartitionTable) {
				continue
			}

			return nil, fmt.Errorf("failed to open partition table: %w", err)
		}

		for _, p := range pt.Partitions() {
			if part, ok := p.(*gptpartition.Partition); ok {
				if part.Name == name {
					return bd, nil
				}
			}
		}

		// nolint: errcheck
		bd.Close()
	}

	return nil, os.ErrNotExist
}

// GetPartitionWithName probes all known block device's partition
// table for a parition with the specified name.
//
//nolint: gocyclo
func GetPartitionWithName(name string) (f *os.File, err error) {
	var infos []os.FileInfo

	if infos, err = ioutil.ReadDir("/sys/block"); err != nil {
		return nil, err
	}

	for _, info := range infos {
		devpath := "/dev/" + info.Name()

		var bd *blockdevice.BlockDevice

		if bd, err = blockdevice.Open(devpath); err != nil {
			continue
		}

		// nolint: errcheck
		defer bd.Close()

		pt, err := bd.PartitionTable()
		if err != nil {
			if errors.Is(err, blockdevice.ErrMissingPartitionTable) {
				continue
			}

			return nil, fmt.Errorf("failed to open partition table: %w", err)
		}

		for _, p := range pt.Partitions() {
			if part, ok := p.(*gptpartition.Partition); ok {
				if part.Name == name {
					partpath, err := util.PartPath(info.Name(), int(part.No()))
					if err != nil {
						return nil, err
					}

					f, err = os.OpenFile(partpath, os.O_RDWR|unix.O_CLOEXEC, os.ModeDevice)
					if err != nil {
						return nil, err
					}

					return f, nil
				}
			}
		}
	}

	return nil, os.ErrNotExist
}

func probeFilesystem(devpath string) (probed []*ProbedBlockDevice, err error) {
	for _, path := range probe(devpath) {
		var (
			bd *blockdevice.BlockDevice
			sb filesystem.SuperBlocker
		)
		// We ignore the error here because there is the
		// possibility that opening the block device fails for
		// good reason (e.g. no partition table, read-only
		// filesystem), but the block device does have a
		// filesystem. This is currently a limitation in our
		// blockdevice package. We should make that package
		// better and update the code here.
		// nolint: errcheck
		bd, _ = blockdevice.Open(devpath)

		if sb, err = FileSystem(path); err != nil {
			return nil, fmt.Errorf("unexpected error when reading super block: %w", err)
		}

		probed = append(probed, &ProbedBlockDevice{BlockDevice: bd, SuperBlock: sb, Path: path})
	}

	return probed, nil
}

func filterByLabel(probed []*ProbedBlockDevice, value string) (probe *ProbedBlockDevice, err error) {
	for _, probe = range probed {
		switch sb := probe.SuperBlock.(type) {
		case *iso9660.SuperBlock:
			trimmed := bytes.Trim(sb.VolumeID[:], " \x00")
			if bytes.Equal(trimmed, []byte(value)) {
				return probe, nil
			}
		case *vfat.SuperBlock:
			trimmed := bytes.Trim(sb.Label[:], " \x00")
			if bytes.Equal(trimmed, []byte(value)) {
				return probe, nil
			}
		case *xfs.SuperBlock:
			trimmed := bytes.Trim(sb.Fname[:], " \x00")
			if bytes.Equal(trimmed, []byte(value)) {
				return probe, nil
			}
		}
	}

	return nil, os.ErrNotExist
}
