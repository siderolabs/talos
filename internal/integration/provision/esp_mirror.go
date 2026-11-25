// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"

	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
)

// mdBootArrayPatch builds a RAID1 boot array (metadata at the end, so that the firmware sees a plain disk)
// over the primary disks and installs Talos to it, same as hack/test/patches/md-boot-array.yaml.
const mdBootArrayPatch = `apiVersion: v1alpha1
kind: RAIDArrayConfig
name: boot
level: raid1
metadata: "1.0"
provisioning:
    volumeSelector:
        match: disk.transport == "virtio" && disk.size >= 12u * GiB
---
apiVersion: v1alpha1
kind: UnattendedInstallConfig
provisioning:
    diskSelector:
        match: '"/dev/disk/by-id/md-name-talos:boot" in disk.symlinks'
`

// mdBootArrayMembers is the number of primary disks the MD boot array is built from.
const mdBootArrayMembers = 2

func (suite *BaseSuite) mdBootArrayPatches() []configpatcher.Patch {
	patch, err := configpatcher.LoadPatch([]byte(mdBootArrayPatch))
	suite.Require().NoError(err)

	return []configpatcher.Patch{patch}
}

// memberDiskPath returns the path of the disk image backing the MD boot array member.
func (suite *BaseSuite) memberDiskPath(node provision.NodeInfo, member int) string {
	statePath, err := suite.Cluster.StatePath()
	suite.Require().NoError(err)

	return filepath.Join(statePath, fmt.Sprintf("%s-%d.disk", node.Name, member))
}

// espRange returns the byte range of the ESP within the MD boot array member disk image.
//
// With metadata 1.0, the array data starts at offset 0 of each member, so the GPT of the array is
// readable directly from the member, and offsets within the array match offsets within the member.
func (suite *BaseSuite) espRange(diskPath string) (offset, length int64) {
	f, err := os.Open(diskPath)
	suite.Require().NoError(err)

	defer f.Close() //nolint:errcheck

	dev, err := gpt.DeviceFromFile(f)
	suite.Require().NoError(err)

	table, err := gpt.Read(dev)
	suite.Require().NoError(err)

	for _, part := range table.Partitions() {
		if part != nil && part.Name == constants.EFIPartitionLabel {
			sectorSize := int64(dev.GetSectorSize())

			return int64(part.FirstLBA) * sectorSize, int64(part.LastLBA-part.FirstLBA+1) * sectorSize
		}
	}

	suite.FailNow("ESP not found", "no partition with label %q in %s", constants.EFIPartitionLabel, diskPath)

	return 0, 0
}

// divergeESPMirror writes to the end of the ESP on the second member of the MD boot array behind the back
// of MD, as the firmware would, so that the copies differ.
//
// The end of the ESP is not allocated by the FAT (it only holds a few files), so the filesystem doesn't
// care about the contents, but the MD repair on upgrade has to bring it back in sync.
func (suite *BaseSuite) divergeESPMirror(node provision.NodeInfo) {
	diskPath := suite.memberDiskPath(node, 1)

	offset, length := suite.espRange(diskPath)

	const divergedSize = 1024 * 1024

	suite.T().Logf("diverging the ESP copy on %s at %d", diskPath, offset+length-divergedSize)

	f, err := os.OpenFile(diskPath, os.O_WRONLY, 0)
	suite.Require().NoError(err)

	defer f.Close() //nolint:errcheck

	_, err = f.WriteAt(bytes.Repeat([]byte{0x55}, divergedSize), offset+length-divergedSize)
	suite.Require().NoError(err)

	suite.Require().NoError(f.Sync())

	suite.Require().False(suite.espMirrored(node), "ESP copies should differ after the out-of-band write")
}

// espMirrored compares the ESP copies on all members of the MD boot array.
func (suite *BaseSuite) espMirrored(node provision.NodeInfo) bool {
	var (
		offset, length int64
		readers        []io.Reader //nolint:prealloc
	)

	for member := range mdBootArrayMembers {
		diskPath := suite.memberDiskPath(node, member)

		memberOffset, memberLength := suite.espRange(diskPath)

		if member == 0 {
			offset, length = memberOffset, memberLength
		} else {
			suite.Require().Equal([2]int64{offset, length}, [2]int64{memberOffset, memberLength}, "ESP location differs between members")
		}

		f, err := os.Open(diskPath)
		suite.Require().NoError(err)

		defer f.Close() //nolint:errcheck

		readers = append(readers, io.NewSectionReader(f, offset, length))
	}

	const chunkSize = 4 * 1024 * 1024

	bufs := make([][]byte, len(readers))
	for i := range bufs {
		bufs[i] = make([]byte, chunkSize)
	}

	for pos := int64(0); pos < length; pos += chunkSize {
		for i, r := range readers {
			n, err := io.ReadFull(r, bufs[i])
			if err != io.ErrUnexpectedEOF {
				suite.Require().NoError(err)
			}

			bufs[i] = bufs[i][:n]
		}

		for i := 1; i < len(bufs); i++ {
			if !bytes.Equal(bufs[0], bufs[i]) {
				suite.T().Logf("ESP copies of %s differ in the chunk at offset %d", node.Name, pos)

				return false
			}
		}
	}

	return true
}

// assertESPMirrored asserts that the ESP copies on all members of the MD boot array are identical.
func (suite *BaseSuite) assertESPMirrored(node provision.NodeInfo) {
	suite.Assert().True(suite.espMirrored(node), "ESP copies of %s differ", node.Name)
}
