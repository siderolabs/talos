// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount_test

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-blockdevice/blockdevice/loopback"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/makefs"
)

// Some tests in this package cannot be run under buildkit, as buildkit doesn't propagate partition devices
// like /dev/loopXpY into the sandbox. To run the tests on your local computer, do the following:
//
//  go test -exec sudo -v --count 1 github.com/talos-systems/talos/internal/pkg/mount

type manifestSuite struct {
	suite.Suite

	disk           *os.File
	loopbackDevice *os.File
}

const (
	diskSize = 4 * 1024 * 1024 * 1024 // 4 GiB
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

func (suite *manifestSuite) TestCleanCorrupedXFSFileSystem() {
	suite.skipUnderBuildkit()

	tempDir := suite.T().TempDir()

	mountDir := filepath.Join(tempDir, "var")

	suite.Assert().NoError(os.MkdirAll(mountDir, 0o700))
	suite.Require().NoError(makefs.XFS(suite.loopbackDevice.Name()))

	logger := log.New(os.Stderr, "", log.LstdFlags)

	mountpoint := mount.NewMountPoint(suite.loopbackDevice.Name(), mountDir, "xfs", unix.MS_NOATIME, "", mount.WithLogger(logger))

	suite.Assert().NoError(mountpoint.Mount())

	defer func() {
		suite.Assert().NoError(mountpoint.Unmount())
	}()

	suite.Assert().NoError(mountpoint.Unmount())

	// // now corrupt the disk
	cmd := exec.Command("xfs_db", []string{
		"-x",
		"-c blockget",
		"-c blocktrash -s 512109 -n 1000",
		suite.loopbackDevice.Name(),
	}...)

	suite.Assert().NoError(cmd.Run())

	suite.Assert().NoError(mountpoint.Mount())
}
