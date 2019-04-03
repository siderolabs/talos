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
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem/iso9660"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem/xfs"
)

// ProbedBlockDevice represents a probed block device.
type ProbedBlockDevice struct {
	*blockdevice.BlockDevice

	SuperBlock filesystem.SuperBlocker
	Path       string
}

// All probes a block device's file system for the given label.
func All() (probed []*ProbedBlockDevice, err error) {
	var infos []os.FileInfo
	if infos, err = ioutil.ReadDir("/sys/block"); err != nil {
		return nil, err
	}

	probe := func(devpath string) (sb filesystem.SuperBlocker) {
		// nolint: errcheck
		sb, _ = FileSystem(devpath)
		return sb
	}

	for _, info := range infos {
		var sb filesystem.SuperBlocker
		devpath := "/dev/" + info.Name()

		bd, err := blockdevice.Open(devpath)
		if err != nil {
			// A partition table was not found, but it is still possible that a
			// file system exists without a partition table.
			if sb = probe(devpath); sb != nil {
				probed = append(probed, &ProbedBlockDevice{BlockDevice: bd, SuperBlock: sb, Path: devpath})
			}
			continue
		}

		pt, err := bd.PartitionTable(true)
		if err != nil {
			// A partition table was not found, and we have already checked for
			// a file system on the block device.
			continue
		}

		// A partition table was found, now probe each partition's file system.
		for _, p := range pt.Partitions() {
			var partpath string
			if strings.HasPrefix(info.Name(), "nvme") {
				partpath = fmt.Sprintf("/dev/%sp%d", info.Name(), p.No())
			} else {
				partpath = fmt.Sprintf("/dev/%s%d", info.Name(), p.No())
			}
			if sb = probe(partpath); sb != nil {
				probed = append(probed, &ProbedBlockDevice{BlockDevice: bd, SuperBlock: sb, Path: partpath})
			}
		}
	}

	return probed, nil
}

// FileSystem probes the provided path's file system.
func FileSystem(path string) (sb filesystem.SuperBlocker, err error) {
	var f *os.File
	if f, err = os.Open(path); err != nil {
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

// GetDevWithFileSystemLabel probes a block device's file system for the given label.
func GetDevWithFileSystemLabel(value string) (probe *ProbedBlockDevice, err error) {
	var probed []*ProbedBlockDevice
	if probed, err = All(); err != nil {
		return nil, err
	}

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
