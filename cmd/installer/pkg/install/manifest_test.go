// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/table/gpt/partition"

	"github.com/talos-systems/talos/cmd/installer/pkg/install"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/loopback"
	"github.com/talos-systems/talos/internal/pkg/mount"
)

// Some tests in this package cannot be run under buildkit, as buildkit doesn't propagate partition devices
// like /dev/loopXpY into the sandbox. To run the tests on your local computer, do the following:
//
//	 sudo go test -v --count 1 ./cmd/installer/pkg/install/

type manifestSuite struct {
	suite.Suite

	disk           *os.File
	loopbackDevice *os.File
}

const (
	diskSize    = 4 * 1024 * 1024 * 1024 // 4 GiB
	lbaSize     = 512
	gptReserved = 67
)

func TestManifestSuite(t *testing.T) {
	suite.Run(t, new(manifestSuite))
}

func (suite *manifestSuite) SetupSuite() {
	suite.skipIfNotRoot()

	var err error

	suite.disk, err = ioutil.TempFile("", "talos")
	suite.Require().NoError(err)

	suite.Require().NoError(suite.disk.Truncate(diskSize))

	suite.loopbackDevice, err = loopback.NextLoopDevice()
	suite.Require().NoError(err)

	suite.T().Logf("Using %s", suite.loopbackDevice.Name())

	suite.Require().NoError(loopback.Loop(suite.loopbackDevice, suite.disk))

	suite.Require().NoError(loopback.LoopSetReadWrite(suite.loopbackDevice))
}

func (suite *manifestSuite) TearDownSuite() {
	if suite.loopbackDevice != nil {
		suite.Assert().NoError(loopback.Unloop(suite.loopbackDevice))
	}

	if suite.disk != nil {
		suite.Assert().NoError(os.Remove(suite.disk.Name()))
		suite.Assert().NoError(suite.disk.Close())
	}
}

func (suite *manifestSuite) skipUnderBuildkit() {
	hostname, _ := os.Hostname() //nolint: errcheck

	if hostname == "buildkitsandbox" {
		suite.T().Skip("test not supported under buildkit as partition devices are not propagated from /dev")
	}
}

func (suite *manifestSuite) skipIfNotRoot() {
	if os.Getuid() != 0 {
		suite.T().Skip("can't run the test as non-root")
	}
}

func (suite *manifestSuite) verifyBlockdevice(manifest *install.Manifest) {
	bd, err := blockdevice.Open(suite.loopbackDevice.Name())
	suite.Require().NoError(err)

	defer bd.Close() //nolint: errcheck

	table, err := bd.PartitionTable()
	suite.Require().NoError(err)

	// verify partition table

	suite.Assert().Len(table.Partitions(), 6)

	part := table.Partitions()[0]
	suite.Assert().Equal(install.EFISystemPartition, strings.ToUpper(part.(*partition.Partition).Type.String()))
	suite.Assert().EqualValues(0, part.(*partition.Partition).Flags)
	suite.Assert().EqualValues(install.EFISize/lbaSize, part.Length())

	part = table.Partitions()[1]
	suite.Assert().Equal(install.BIOSBootPartition, strings.ToUpper(part.(*partition.Partition).Type.String()))
	suite.Assert().EqualValues(4, part.(*partition.Partition).Flags)
	suite.Assert().EqualValues(install.BIOSGrubSize/lbaSize, part.Length())

	part = table.Partitions()[2]
	suite.Assert().Equal(install.LinuxFilesystemData, strings.ToUpper(part.(*partition.Partition).Type.String()))
	suite.Assert().EqualValues(0, part.(*partition.Partition).Flags)
	suite.Assert().EqualValues(install.BootSize/lbaSize, part.Length())

	part = table.Partitions()[3]
	suite.Assert().Equal(install.LinuxFilesystemData, strings.ToUpper(part.(*partition.Partition).Type.String()))
	suite.Assert().EqualValues(0, part.(*partition.Partition).Flags)
	suite.Assert().EqualValues(install.MetaSize/lbaSize, part.Length())

	part = table.Partitions()[4]
	suite.Assert().Equal(install.LinuxFilesystemData, strings.ToUpper(part.(*partition.Partition).Type.String()))
	suite.Assert().EqualValues(0, part.(*partition.Partition).Flags)
	suite.Assert().EqualValues(install.StateSize/lbaSize, part.Length())

	part = table.Partitions()[5]
	suite.Assert().Equal(install.LinuxFilesystemData, strings.ToUpper(part.(*partition.Partition).Type.String()))
	suite.Assert().EqualValues(0, part.(*partition.Partition).Flags)
	suite.Assert().EqualValues((diskSize-install.EFISize-install.BIOSGrubSize-install.BootSize-install.MetaSize-install.StateSize)/lbaSize-gptReserved, part.Length())

	suite.Assert().NoError(bd.Close())

	// query mount points directly for the device

	mountpoints, err := mount.SystemMountPointsForDevice(suite.loopbackDevice.Name())
	suite.Require().NoError(err)

	suite.Assert().Equal(4, mountpoints.Len())

	// verify filesystems by mounting and unmounting

	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer func() {
		suite.Assert().NoError(os.RemoveAll(tempDir))
	}()

	mountpoints, err = manifest.SystemMountpoints()
	suite.Require().NoError(err)

	suite.Assert().Equal(4, mountpoints.Len())

	suite.Require().NoError(mount.PrefixMountTargets(mountpoints, tempDir))

	err = mount.Mount(mountpoints)
	suite.Require().NoError(err)

	defer func() {
		suite.Assert().NoError(mount.Unmount(mountpoints))
	}()
}

func (suite *manifestSuite) TestExecuteManifestClean() {
	suite.skipUnderBuildkit()

	manifest, err := install.NewManifest("A", runtime.SequenceInstall, &install.Options{
		Disk:       suite.loopbackDevice.Name(),
		Bootloader: true,
		Force:      true,
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(manifest.Execute())

	suite.verifyBlockdevice(manifest)
}

func (suite *manifestSuite) TestExecuteManifestForce() {
	suite.skipUnderBuildkit()

	manifest, err := install.NewManifest("A", runtime.SequenceInstall, &install.Options{
		Disk:       suite.loopbackDevice.Name(),
		Bootloader: true,
		Force:      true,
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(manifest.Execute())

	// reinstall

	manifest, err = install.NewManifest("B", runtime.SequenceInstall, &install.Options{
		Disk:       suite.loopbackDevice.Name(),
		Bootloader: true,
		Force:      true,
		Zero:       true,
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(manifest.Execute())

	suite.verifyBlockdevice(manifest)
}

func (suite *manifestSuite) TestTargetInstall() {
	// Create Temp dirname for mountpoint
	dir, err := ioutil.TempDir("", "talostest")
	suite.Require().NoError(err)

	// nolint: errcheck
	defer os.RemoveAll(dir)

	// Create a tempfile for local copy
	src, err := ioutil.TempFile(dir, "example")
	suite.Require().NoError(err)

	suite.Require().NoError(src.Close())

	dst := filepath.Join(dir, "dest")

	// Attempt to download and copy files
	target := &install.Target{
		Assets: []*install.Asset{
			{
				Source:      src.Name(),
				Destination: dst,
			},
		},
	}

	suite.Require().NoError(target.Save())

	for _, expectedFile := range target.Assets {
		// Verify copied file is at the appropriate location.
		_, err := os.Stat(expectedFile.Destination)
		suite.Require().NoError(err)
	}
}
