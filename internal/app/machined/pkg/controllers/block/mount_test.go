// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/xfs"
)

type MountSuite struct {
	ctest.DefaultSuite
}

func TestMountSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &MountSuite{
		Timeout: 30 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.MountController{}))
		},
	})
}

func (suite *MountSuite) mountVolume(volumeID string) {
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

// TestMountFailureDoesNotBlockOthers verifies that a failed mount is retried without holding back the mount
// requests sorted after it.
func (suite *MountSuite) TestMountFailureDoesNotBlockOthers() {
	dir := suite.T().TempDir()

	// without Force, a file at the target fails the symlink mount
	failingPath := filepath.Join(dir, "failing")
	suite.Require().NoError(os.WriteFile(failingPath, nil, 0o644))

	for _, volume := range []struct {
		id   string
		path string
	}{
		{id: "a-failing", path: failingPath},
		{id: "b-other", path: filepath.Join(dir, "other")},
	} {
		volumeStatus := block.NewVolumeStatus(block.NamespaceName, volume.id)
		volumeStatus.TypedSpec().Type = block.VolumeTypeSymlink
		volumeStatus.TypedSpec().SymlinkSpec = block.SymlinkProvisioningSpec{SymlinkTargetPath: "/run"}
		volumeStatus.TypedSpec().MountSpec = block.MountSpec{TargetPath: volume.path}
		volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
		suite.Create(volumeStatus)
	}

	mountRequest := block.NewMountRequest(block.NamespaceName, "a-failing")
	mountRequest.TypedSpec().RequesterIDs = []string{"requester1/a-failing"}
	mountRequest.TypedSpec().Requesters = []string{"requester1"}
	mountRequest.TypedSpec().VolumeID = "a-failing"
	suite.Create(mountRequest)

	suite.mountVolume("b-other")

	ctest.AssertNoResource[*block.MountStatus](suite, "a-failing")

	suite.Require().NoError(os.Remove(failingPath))

	// any input event retries the mount, well before the retry interval
	suite.Create(block.NewVolumeStatus(block.NamespaceName, "c-trigger"))

	ctest.AssertResource(suite, "a-failing", func(*block.MountStatus, *assert.Assertions) {})

	target, err := os.Readlink(failingPath)
	suite.Require().NoError(err)
	suite.Assert().Equal("/run", target)
}

func (suite *MountSuite) TestUserVolumeMountPointCleanup() {
	dir := suite.T().TempDir()

	// leftover empty mount point
	suite.Require().NoError(os.Mkdir(filepath.Join(dir, "empty"), 0o755))
	// non-empty directory
	suite.Require().NoError(os.Mkdir(filepath.Join(dir, "data"), 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(dir, "data", "file"), []byte("data"), 0o644))
	// regular file
	suite.Require().NoError(os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o644))

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, constants.UserVolumeMountPoint)
	volumeStatus.TypedSpec().Type = block.VolumeTypeDirectory
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{
		TargetPath: dir,
		FileMode:   0o755,
		UID:        os.Getuid(),
		GID:        os.Getgid(),
	}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	suite.mountVolume(constants.UserVolumeMountPoint)

	suite.Assert().NoDirExists(filepath.Join(dir, "empty"))
	suite.Assert().FileExists(filepath.Join(dir, "data", "file"))
	suite.Assert().FileExists(filepath.Join(dir, "file"))
}

// TestUnmountBusy verifies that a busy unmount neither holds back the teardown of other mounts,
// nor waits for the controller restart backoff once released.
func (suite *MountSuite) TestUnmountBusy() {
	if os.Getuid() != 0 {
		suite.T().Skip("can't run the test as non-root")
	}

	dir := suite.T().TempDir()

	// mount requests are processed in ID order, so the busy one goes first
	for _, id := range []string{"a-busy", "b-other"} {
		volumeStatus := block.NewVolumeStatus(block.NamespaceName, id)
		volumeStatus.TypedSpec().Type = block.VolumeTypeDirectory
		volumeStatus.TypedSpec().MountSpec = block.MountSpec{
			TargetPath: filepath.Join(dir, id),
			BindTarget: new(filepath.Join(dir, id+"-bind")),
			FileMode:   0o755,
		}
		volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
		suite.Create(volumeStatus)

		suite.mountVolume(id)
	}

	// an open directory keeps the bind mount busy
	busy, err := os.Open(filepath.Join(dir, "a-busy-bind"))
	suite.Require().NoError(err)

	for _, id := range []string{"a-busy", "b-other"} {
		_, err = suite.State().Teardown(suite.Ctx(), block.NewMountRequest(block.NamespaceName, id).Metadata())
		suite.Require().NoError(err)
	}

	// each busy unmount attempt blocks for ~5s retrying EBUSY, and "b-other" may wait behind two of them:
	// one in an iteration that ran before its teardown, one in the iteration handling it
	ctx, cancel := context.WithTimeout(suite.Ctx(), 20*time.Second)
	defer cancel()

	rtestutils.AssertResource(ctx, suite.T(), suite.State(), "b-other", func(mountRequest *block.MountRequest, asrt *assert.Assertions) {
		asrt.True(mountRequest.Metadata().Finalizers().Empty())
	})

	ctest.AssertResource(suite, "a-busy", func(mountRequest *block.MountRequest, asrt *assert.Assertions) {
		asrt.False(mountRequest.Metadata().Finalizers().Empty())
	})

	suite.Require().NoError(busy.Close())

	// any input event retries the unmount, well before the retry interval
	suite.Create(block.NewVolumeStatus(block.NamespaceName, "c-trigger"))

	ctest.AssertResource(suite, "a-busy", func(mountRequest *block.MountRequest, asrt *assert.Assertions) {
		asrt.True(mountRequest.Metadata().Finalizers().Empty())
	})
}

// createTmpfsVolume creates a ready tmpfs VolumeStatus mounted at targetPath.
func (suite *MountSuite) createTmpfsVolume(id, targetPath string, parameters ...block.ParameterSpec) *block.VolumeStatus {
	volumeStatus := block.NewVolumeStatus(block.NamespaceName, id)
	volumeStatus.TypedSpec().Type = block.VolumeTypeTmpfs
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{
		TargetPath: targetPath,
		FileMode:   0o755,
		Parameters: parameters,
	}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	return volumeStatus
}

// assertTmpfs waits for the tmpfs mounted at targetPath to report the expected statfs values.
func (suite *MountSuite) assertTmpfs(targetPath string, check func(*assert.CollectT, unix.Statfs_t)) {
	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		var st unix.Statfs_t

		if !assert.NoError(collect, unix.Statfs(targetPath, &st)) {
			return
		}

		if !assert.Equal(collect, uint32(unix.TMPFS_MAGIC), uint32(st.Type)) {
			return
		}

		check(collect, st)
	}, 10*time.Second, 50*time.Millisecond)
}

// assertTmpfsSize waits for the tmpfs mounted at targetPath to have the expected size in bytes.
func (suite *MountSuite) assertTmpfsSize(targetPath string, expected int64) {
	suite.assertTmpfs(targetPath, func(collect *assert.CollectT, st unix.Statfs_t) {
		assert.Equal(collect, expected, int64(st.Blocks)*st.Bsize)
	})
}

// setReadOnly updates the read-only flag of the mount request.
func (suite *MountSuite) setReadOnly(mountRequestID string, readOnly bool) {
	ctest.UpdateWithConflicts(suite, block.NewMountRequest(block.NamespaceName, mountRequestID), func(mr *block.MountRequest) error {
		mr.TypedSpec().ReadOnly = readOnly

		return nil
	})
}

// unmountVolume tears down the mount request and waits for the controller to release it.
func (suite *MountSuite) unmountVolume(volumeID string) {
	_, err := suite.State().Teardown(suite.Ctx(), block.NewMountRequest(block.NamespaceName, volumeID).Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, volumeID, func(mountRequest *block.MountRequest, asrt *assert.Assertions) {
		asrt.True(mountRequest.Metadata().Finalizers().Empty())
	})
}

func (suite *MountSuite) TestTmpfsRequiresSize() {
	for _, test := range []struct {
		id         string
		parameters []block.ParameterSpec
	}{
		{
			id: "tmpfs-no-size",
		},
		{
			id:         "tmpfs-other-parameters",
			parameters: []block.ParameterSpec{block.NewStringParameter("nr_inodes", "1024")},
		},
		{
			id:         "tmpfs-nil-size",
			parameters: []block.ParameterSpec{{Type: block.FSParameterTypeStringValue, Name: block.TmpfsSizeParameter}},
		},
		{
			id: "tmpfs-mode",
			parameters: []block.ParameterSpec{
				block.NewStringParameter(block.TmpfsSizeParameter, "1048576"),
				block.NewStringParameter("mode", "0700"),
			},
		},
	} {
		suite.Run(test.id, func() {
			targetPath := filepath.Join(suite.T().TempDir(), "tmpfs")

			suite.createTmpfsVolume(test.id, targetPath, test.parameters...)

			mountRequest := block.NewMountRequest(block.NamespaceName, test.id)
			mountRequest.TypedSpec().RequesterIDs = []string{"requester1/" + test.id}
			mountRequest.TypedSpec().Requesters = []string{"requester1"}
			mountRequest.TypedSpec().VolumeID = test.id
			suite.Create(mountRequest)

			// the finalizer is added right before the mount operation, so the controller has reached it
			ctest.AssertResource(suite, test.id, func(volumeStatus *block.VolumeStatus, asrt *assert.Assertions) {
				asrt.True(volumeStatus.Metadata().Finalizers().Has("block.MountController"))
			})

			suite.Assert().Never(func() bool {
				_, err := suite.State().Get(suite.Ctx(), block.NewMountStatus(block.NamespaceName, test.id).Metadata())

				return err == nil
			}, time.Second, 50*time.Millisecond)

			_, err := os.Stat(targetPath)
			suite.Require().ErrorIs(err, os.ErrNotExist)
		})
	}

	// the invalid tmpfs volumes don't hold back a mount request sorted after them
	symlinkPath := filepath.Join(suite.T().TempDir(), "symlink")

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "tmpfs-zz-symlink")
	volumeStatus.TypedSpec().Type = block.VolumeTypeSymlink
	volumeStatus.TypedSpec().SymlinkSpec = block.SymlinkProvisioningSpec{SymlinkTargetPath: "/run"}
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{TargetPath: symlinkPath}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	suite.mountVolume("tmpfs-zz-symlink")

	target, err := os.Readlink(symlinkPath)
	suite.Require().NoError(err)
	suite.Assert().Equal("/run", target)
}

// TestTmpfsReadOnlyRemount verifies the read-only toggle of a tmpfs mount, which goes through the generic remount path.
func (suite *MountSuite) TestTmpfsReadOnlyRemount() {
	if os.Getuid() != 0 {
		suite.T().Skip("can't run the test as non-root")
	}

	targetPath := filepath.Join(suite.T().TempDir(), "tmpfs")

	suite.createTmpfsVolume("tmpfs-ro", targetPath, block.NewStringParameter(block.TmpfsSizeParameter, "1048576"))

	suite.mountVolume("tmpfs-ro")
	suite.Require().NoError(os.WriteFile(filepath.Join(targetPath, "file"), []byte("content"), 0o644))

	suite.setReadOnly("tmpfs-ro", true)

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assert.ErrorIs(collect, os.WriteFile(filepath.Join(targetPath, "other"), nil, 0o644), unix.EROFS)
	}, 10*time.Second, 50*time.Millisecond)

	// the remount keeps the superblock
	suite.Assert().FileExists(filepath.Join(targetPath, "file"))

	suite.setReadOnly("tmpfs-ro", false)

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assert.NoError(collect, os.WriteFile(filepath.Join(targetPath, "other"), nil, 0o644))
	}, 10*time.Second, 50*time.Millisecond)

	suite.unmountVolume("tmpfs-ro")
}

// TestTmpfsReadOnlyAppliesSettings verifies that a tmpfs mounted read-only from the start still gets
// the mode and owner of its mount spec, as a new tmpfs has no metadata to preserve.
func (suite *MountSuite) TestTmpfsReadOnlyAppliesSettings() {
	if os.Getuid() != 0 {
		suite.T().Skip("can't run the test as non-root")
	}

	targetPath := filepath.Join(suite.T().TempDir(), "tmpfs")

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, "tmpfs-ro-settings")
	volumeStatus.TypedSpec().Type = block.VolumeTypeTmpfs
	volumeStatus.TypedSpec().MountSpec = block.MountSpec{
		TargetPath: targetPath,
		FileMode:   0o700,
		UID:        1234,
		GID:        5678,
		Parameters: []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "1048576")},
	}
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)

	mountRequest := block.NewMountRequest(block.NamespaceName, "tmpfs-ro-settings")
	mountRequest.TypedSpec().RequesterIDs = []string{"requester1/tmpfs-ro-settings"}
	mountRequest.TypedSpec().Requesters = []string{"requester1"}
	mountRequest.TypedSpec().VolumeID = "tmpfs-ro-settings"
	mountRequest.TypedSpec().ReadOnly = true
	suite.Create(mountRequest)

	ctest.AssertResource(suite, "tmpfs-ro-settings", func(mountStatus *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(mountStatus.TypedSpec().ReadOnly)
	})

	assertSettings := func() {
		var st unix.Stat_t

		suite.Require().NoError(unix.Stat(targetPath, &st))
		suite.Assert().EqualValues(0o700, st.Mode&0o7777)
		suite.Assert().EqualValues(1234, st.Uid)
		suite.Assert().EqualValues(5678, st.Gid)
	}

	assertSettings()
	suite.Assert().ErrorIs(os.WriteFile(filepath.Join(targetPath, "file"), nil, 0o644), unix.EROFS)

	suite.setReadOnly("tmpfs-ro-settings", false)

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assert.NoError(collect, os.WriteFile(filepath.Join(targetPath, "file"), nil, 0o644))
	}, 10*time.Second, 50*time.Millisecond)

	assertSettings()

	suite.unmountVolume("tmpfs-ro-settings")
}

// TestTmpfsParametersNotReconfigured verifies that a change of the parameters of a mounted tmpfs is not applied
// to the mount, which keeps its content: the change takes effect on the next mount.
func (suite *MountSuite) TestTmpfsParametersNotReconfigured() {
	if os.Getuid() != 0 {
		suite.T().Skip("can't run the test as non-root")
	}

	targetPath := filepath.Join(suite.T().TempDir(), "tmpfs")

	volumeStatus := suite.createTmpfsVolume("tmpfs-frozen", targetPath, block.NewStringParameter(block.TmpfsSizeParameter, "1048576"))

	suite.mountVolume("tmpfs-frozen")
	suite.assertTmpfsSize(targetPath, 1<<20)

	suite.Require().NoError(os.WriteFile(filepath.Join(targetPath, "file"), []byte("content"), 0o644))

	mountStatus, err := safe.StateGetByID[*block.MountStatus](suite.Ctx(), suite.State(), "tmpfs-frozen")
	suite.Require().NoError(err)

	ctest.UpdateWithConflicts(suite, volumeStatus, func(vs *block.VolumeStatus) error {
		vs.TypedSpec().MountSpec.Parameters = []block.ParameterSpec{block.NewStringParameter(block.TmpfsSizeParameter, "2097152")}

		return nil
	})

	// neither resized, nor remounted
	suite.Assert().Never(func() bool {
		var st unix.Statfs_t

		if err := unix.Statfs(targetPath, &st); err != nil || int64(st.Blocks)*st.Bsize != 1<<20 {
			return true
		}

		current, err := suite.State().Get(suite.Ctx(), mountStatus.Metadata())

		return err != nil || !current.Metadata().Created().Equal(mountStatus.Metadata().Created())
	}, time.Second, 50*time.Millisecond)

	suite.Assert().FileExists(filepath.Join(targetPath, "file"))

	// the next mount applies the change
	suite.unmountVolume("tmpfs-frozen")
	suite.Destroy(block.NewMountRequest(block.NamespaceName, "tmpfs-frozen"))

	suite.mountVolume("tmpfs-frozen")
	suite.assertTmpfsSize(targetPath, 2<<20)

	suite.unmountVolume("tmpfs-frozen")
}

// TestTmpfsDetached verifies a detached tmpfs mount: it is never attached to the target, and it is
// reachable through its mount file descriptor.
func (suite *MountSuite) TestTmpfsDetached() {
	if os.Getuid() != 0 {
		suite.T().Skip("can't run the test as non-root")
	}

	targetPath := filepath.Join(suite.T().TempDir(), "tmpfs")

	suite.createTmpfsVolume("tmpfs-detached", targetPath, block.NewStringParameter(block.TmpfsSizeParameter, "1048576"))

	mountRequest := block.NewMountRequest(block.NamespaceName, "tmpfs-detached")
	mountRequest.TypedSpec().RequesterIDs = []string{"requester1/tmpfs-detached"}
	mountRequest.TypedSpec().Requesters = []string{"requester1"}
	mountRequest.TypedSpec().VolumeID = "tmpfs-detached"
	mountRequest.TypedSpec().Detached = true
	suite.Create(mountRequest)

	ctest.AssertResource(suite, "tmpfs-detached", func(*block.MountStatus, *assert.Assertions) {})

	if st, err := os.Stat(targetPath); err == nil {
		var statfs unix.Statfs_t

		suite.Require().NoError(unix.Statfs(targetPath, &statfs))
		suite.Assert().NotEqual(uint32(unix.TMPFS_MAGIC), uint32(statfs.Type), "detached mount attached at %q (%v)", targetPath, st.Mode())
	}

	mountStatus, err := safe.StateGetByID[*block.MountStatus](suite.Ctx(), suite.State(), "tmpfs-detached")
	suite.Require().NoError(err)

	root, ok := mountStatus.TypedSpec().Root().(xfs.Root)
	suite.Require().True(ok)

	fd, err := root.Fd()
	suite.Require().NoError(err)

	var st unix.Statfs_t

	suite.Require().NoError(unix.Fstatfs(fd, &st))
	suite.Assert().Equal(uint32(unix.TMPFS_MAGIC), uint32(st.Type))
	suite.Assert().Equal(int64(1<<20), int64(st.Blocks)*st.Bsize)

	// the controller keeps going: a mount request sorted after the detached one is still mounted
	otherPath := filepath.Join(suite.T().TempDir(), "other")

	suite.createTmpfsVolume("tmpfs-detached-other", otherPath, block.NewStringParameter(block.TmpfsSizeParameter, "1048576"))
	suite.mountVolume("tmpfs-detached-other")
	suite.assertTmpfsSize(otherPath, 1<<20)

	suite.unmountVolume("tmpfs-detached-other")
	suite.unmountVolume("tmpfs-detached")
}
