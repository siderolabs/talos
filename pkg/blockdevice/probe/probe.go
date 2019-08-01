/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package probe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/iso9660"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/xfs"
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
	if f, err = os.OpenFile(path, os.O_RDONLY, os.ModeDevice); err != nil {
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
	pt, err := bd.PartitionTable(true)
	if err != nil {
		return devpaths
	}

	// A partition table was found, now probe each partition's file system.
	name := filepath.Base(devpath)
	for _, p := range pt.Partitions() {
		var partpath string
		switch {
		case strings.HasPrefix(name, "nvme"):
			fallthrough
		case strings.HasPrefix(name, "loop"):
			partpath = fmt.Sprintf("/dev/%sp%d", name, p.No())
		default:
			partpath = fmt.Sprintf("/dev/%s%d", name, p.No())
		}
		// nolint: errcheck
		if sb, _ := FileSystem(partpath); sb != nil {
			devpaths = append(devpaths, partpath)
		}
	}

	return devpaths
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
			return nil, errors.Wrap(err, "unexpected error when reading super block")
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

	return nil, errors.Errorf("no device found with label %s", value)
}
