// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type MountSuite struct {
	ctest.DefaultSuite
}

func TestMountSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &MountSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.MountController{}))
			},
		},
	})
}

func (suite *MountSuite) mountVolume(volumeID string) { //nolint:unparam
	mountRequest := block.NewMountRequest(block.NamespaceName, volumeID)
	mountRequest.TypedSpec().RequesterIDs = []string{"requester1/" + volumeID}
	mountRequest.TypedSpec().Requesters = []string{"requester1"}
	mountRequest.TypedSpec().VolumeID = volumeID
	suite.Create(mountRequest)

	// wait for the mount status to be created
	ctest.AssertResource(suite, volumeID, func(*block.MountStatus, *assert.Assertions) {})
}

func (suite *MountSuite) TestSymlinkNew() {
	dir := suite.T().TempDir()
	targetPath := filepath.Join(dir, "target")

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "volume1")
	volumeStatus.TypedSpec().Type = block.VolumeTypeSymlink
	volumeStatus.TypedSpec().SymlinkSpec = block.SymlinkProvisioningSpec{
		SymlinkTargetPath: "/run",
		Force:             true,
	}
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{
		TargetPath: targetPath,
	}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	suite.mountVolume("volume1")

	// verify symlink
	path, err := os.Readlink(targetPath)
	suite.Require().NoError(err)
	suite.Assert().Equal("/run", path)
}

func (suite *MountSuite) TestSymlinkExists() {
	dir := suite.T().TempDir()
	targetPath := filepath.Join(dir, "target")

	// symlink already exists
	suite.Require().NoError(os.Symlink("/run", targetPath))

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "volume1")
	volumeStatus.TypedSpec().Type = block.VolumeTypeSymlink
	volumeStatus.TypedSpec().SymlinkSpec = block.SymlinkProvisioningSpec{
		SymlinkTargetPath: "/run",
	}
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{
		TargetPath: targetPath,
	}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	suite.mountVolume("volume1")

	// verify symlink
	path, err := os.Readlink(targetPath)
	suite.Require().NoError(err)
	suite.Assert().Equal("/run", path)
}

func (suite *MountSuite) TestSymlinkWrong() {
	dir := suite.T().TempDir()
	targetPath := filepath.Join(dir, "target")

	// wrong symlink target
	suite.Require().NoError(os.Symlink("/foo", targetPath))

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "volume1")
	volumeStatus.TypedSpec().Type = block.VolumeTypeSymlink
	volumeStatus.TypedSpec().SymlinkSpec = block.SymlinkProvisioningSpec{
		SymlinkTargetPath: "/run",
		Force:             true,
	}
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{
		TargetPath: targetPath,
	}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	suite.mountVolume("volume1")

	// verify symlink
	path, err := os.Readlink(targetPath)
	suite.Require().NoError(err)
	suite.Assert().Equal("/run", path)
}

func (suite *MountSuite) TestSymlinkDirectory() {
	dir := suite.T().TempDir()
	targetPath := filepath.Join(dir, "target")

	// non-empty directory structure
	suite.Require().NoError(os.Mkdir(targetPath, 0o755))
	suite.Require().NoError(os.Mkdir(filepath.Join(targetPath, "foo"), 0o755))

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "volume1")
	volumeStatus.TypedSpec().Type = block.VolumeTypeSymlink
	volumeStatus.TypedSpec().SymlinkSpec = block.SymlinkProvisioningSpec{
		SymlinkTargetPath: "/run",
		Force:             true,
	}
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{
		TargetPath: targetPath,
	}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	suite.mountVolume("volume1")

	// verify symlink
	path, err := os.Readlink(targetPath)
	suite.Require().NoError(err)
	suite.Assert().Equal("/run", path)
}
