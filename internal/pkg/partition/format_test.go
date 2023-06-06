// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package partition provides common utils for system partition format.
package partition_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-blockdevice/blockdevice/loopback"
	"github.com/siderolabs/go-blockdevice/blockdevice/partition/gpt"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/pkg/partition"
)

type manifestSuite struct {
	suite.Suite

	disk           *os.File
	loopbackDevice *os.File
}

const (
	diskSize = 10 * 1024 * 1024 // 10 MiB
)

func TestManifestSuite(t *testing.T) {
	suite.Run(t, new(manifestSuite))
}

func (suite *manifestSuite) SetupTest() {
	suite.skipIfNotRoot()

	var err error

	suite.disk, err = os.CreateTemp("", "talos")
	suite.Require().NoError(err)

	suite.Require().NoError(suite.disk.Truncate(diskSize))

	suite.loopbackDevice, err = loopback.NextLoopDevice()
	suite.Require().NoError(err)

	suite.T().Logf("Using %s", suite.loopbackDevice.Name())

	suite.Require().NoError(loopback.Loop(suite.loopbackDevice, suite.disk))

	suite.Require().NoError(loopback.LoopSetReadWrite(suite.loopbackDevice))
}

func (suite *manifestSuite) TearDownTest() {
	if suite.loopbackDevice != nil {
		suite.Assert().NoError(loopback.Unloop(suite.loopbackDevice))
	}

	if suite.disk != nil {
		suite.Assert().NoError(os.Remove(suite.disk.Name()))
		suite.Assert().NoError(suite.disk.Close())
	}
}

func (suite *manifestSuite) skipIfNotRoot() {
	if os.Getuid() != 0 {
		suite.T().Skip("can't run the test as non-root")
	}
}

func (suite *manifestSuite) skipUnderBuildkit() {
	hostname, _ := os.Hostname() //nolint:errcheck

	if hostname == "buildkitsandbox" {
		suite.T().Skip("test not supported under buildkit as partition devices are not propagated from /dev")
	}
}

func (suite *manifestSuite) TestZeroPartition() {
	suite.skipUnderBuildkit()

	bd, err := blockdevice.Open(suite.loopbackDevice.Name(), blockdevice.WithExclusiveLock(true))
	suite.Require().NoError(err)

	defer bd.Close() //nolint:errcheck

	pt, err := gpt.New(bd.Device(), gpt.WithMarkMBRBootable(false))
	suite.Require().NoError(err)

	// Create a partition table with a single partition.
	_, err = pt.Add(0, gpt.WithMaximumSize(true), gpt.WithPartitionName("zerofill"))
	suite.Require().NoError(err)

	suite.Require().NoError(pt.Write())
	suite.Require().NoError(bd.Close())

	bd, err = blockdevice.Open(suite.loopbackDevice.Name(), blockdevice.WithExclusiveLock(true))
	suite.Require().NoError(err)

	defer bd.Close() //nolint:errcheck

	fills := bytes.NewBuffer(bytes.Repeat([]byte{1}, 10))

	parts, err := bd.GetPartition("zerofill")
	suite.Require().NoError(err)

	part, err := parts.Path()
	suite.Require().NoError(err)

	// open the partition as read write
	dst, err := os.OpenFile(part, os.O_WRONLY, 0o644)
	suite.Require().NoError(err)

	defer dst.Close() //nolint:errcheck

	// Write some data to the partition.
	_, err = io.Copy(dst, fills)
	suite.Require().NoError(err)

	data, err := os.Open(part)
	suite.Require().NoError(err)

	defer data.Close() //nolint:errcheck

	read := make([]byte, fills.Len())

	_, err = data.Read(read)
	suite.Require().NoError(err)
	suite.Require().NoError(data.Close())

	suite.Assert().True(bytes.Equal(fills.Bytes(), read))

	suite.Require().NoError(bd.Close())

	err = partition.Format(part, &partition.FormatOptions{
		FileSystemType: partition.FilesystemTypeNone,
	})
	suite.Require().NoError(err)

	// reading 10 times more than what we wrote should still return 0 since the partition is wiped
	zerofills := bytes.NewBuffer(bytes.Repeat([]byte{0}, 100))

	data, err = os.Open(part)
	suite.Require().NoError(err)

	defer data.Close() //nolint:errcheck

	read = make([]byte, zerofills.Len())

	_, err = data.Read(read)
	suite.Require().NoError(err)

	suite.Assert().True(bytes.Equal(zerofills.Bytes(), read))
}
