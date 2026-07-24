// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// DiskResizeSuite ...
type DiskResizeSuite struct {
	BaseSuite

	track int
}

// SuiteName ...
func (suite *DiskResizeSuite) SuiteName() string {
	return fmt.Sprintf("provision.UpgradeSuite.DiskResize-TR%d", suite.track)
}

// TestResize verifies that Talos can grow partitions set to grow in live mode.
//
//nolint:gocyclo,cyclop
func (suite *DiskResizeSuite) TestResize() {
	sourceInstallerImage := fmt.Sprintf(
		"%s/%s:%s",
		DefaultSettings.TargetInstallImageRegistry,
		images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
		DefaultSettings.CurrentVersion,
	)

	userVolumeConfig := blockcfg.NewUserVolumeConfigV1Alpha1()
	userVolumeConfig.MetaName = "disk"
	userVolumeConfig.VolumeType = new(block.VolumeTypeDisk)
	userVolumeConfig.ProvisioningSpec = blockcfg.ProvisioningSpec{
		DiskSelectorSpec: blockcfg.DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.dev_path == "/dev/sda"`, celenv.DiskLocator())),
		},
		ProvisioningGrow: new(true),
	}
	userVolumeConfig.FilesystemSpec = blockcfg.FilesystemSpec{
		FilesystemType: block.FilesystemTypeEXT4,
	}

	userVolumeCfg, err := container.New(userVolumeConfig)
	suite.Require().NoError(err)

	suite.setupCluster(clusterOptions{
		ClusterName: "disk-resize",

		ControlplaneNodes: 1, // just a single node

		SourceKernelPath:     helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath:  helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: sourceInstallerImage,
		SourceVersion:        DefaultSettings.CurrentVersion,
		SourceK8sVersion:     constants.DefaultKubernetesVersion,

		VersionContract: config.TalosVersionCurrent.DisableEtcd().DisableKubernetes(),
		ConfigPatchesControlPlane: []configpatcher.Patch{
			configpatcher.NewStrategicMergePatch(userVolumeCfg),
		},

		ExtraDisks: 1,
	})

	cli, err := suite.clusterAccess.Client()
	suite.Require().NoError(err)

	ctx := client.WithNode(suite.ctx, suite.clusterAccess.Nodes()[0].IPs[0].String())

	suite.Run("resize ephemeral partition", func() {
		// verify current settings of the partition
		ephemeralStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, cli.COSI, constants.EphemeralPartitionLabel)
		suite.Require().NoError(err)

		preSize := ephemeralStatus.TypedSpec().Size

		suite.T().Logf("EPHEMERAL partition size before resize: %s", humanize.Bytes(preSize))

		fsSize := suite.mountSize(ctx, cli, constants.EphemeralMountPoint)

		suite.T().Logf("EPHEMERAL partition filesystem size before resize: %s", humanize.Bytes(fsSize))

		const growthSize = 10 * 1000 * 1000 * 1000 // 10GB

		suite.growDiskAndWatch(ctx, cli, suite.Cluster.Info().Nodes[0].Name, "vda", 0, "virtio0", growthSize)

		// the EPHEMERAL partition should be grown live
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			ephemeralStatus, err = safe.StateGetByID[*block.VolumeStatus](ctx, cli.COSI, constants.EphemeralPartitionLabel)
			if !assert.NoError(collect, err) {
				return
			}

			assert.Greater(collect, ephemeralStatus.TypedSpec().Size, preSize)
		}, time.Minute, time.Second)

		suite.T().Logf("EPHEMERAL partition size after resize: %s", humanize.Bytes(ephemeralStatus.TypedSpec().Size))

		// ... and the filesystem as well
		var newFsSize uint64

		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			newFsSize = suite.mountSize(ctx, cli, constants.EphemeralMountPoint)

			assert.Greater(collect, newFsSize, fsSize)
		}, time.Minute, time.Second)

		suite.T().Logf("EPHEMERAL partition filesystem size after resize: %s", humanize.Bytes(newFsSize))
	})

	suite.Run("resize user disk volume", func() {
		// verify current settings of the user volume
		const userVolumeID = "u-disk"

		userVolumeMountPoint := filepath.Join(constants.UserVolumeMountPoint, "disk")

		rtestutils.AssertResource[*block.VolumeStatus](ctx, suite.T(), cli.COSI, userVolumeID, func(status *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
		})

		volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, cli.COSI, userVolumeID)
		suite.Require().NoError(err)

		preSize := volumeStatus.TypedSpec().Size

		suite.T().Logf("user volume disk size before resize: %s", humanize.Bytes(preSize))

		fsSize := suite.mountSize(ctx, cli, userVolumeMountPoint)

		suite.T().Logf("user volume filesystem size before resize: %s", humanize.Bytes(fsSize))

		const growthSize = 20 * 1000 * 1000 * 1000 // 20GB

		suite.growDiskAndWatch(ctx, cli, suite.Cluster.Info().Nodes[0].Name, "sda", 1, "scsi1", growthSize)

		// the disk should be grown live
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			volumeStatus, err = safe.StateGetByID[*block.VolumeStatus](ctx, cli.COSI, userVolumeID)
			if !assert.NoError(collect, err) {
				return
			}

			assert.Greater(collect, volumeStatus.TypedSpec().Size, preSize)
		}, time.Minute, time.Second)

		suite.T().Logf("user volume disk size after resize: %s", humanize.Bytes(volumeStatus.TypedSpec().Size))

		// ... and the filesystem as well
		var newFsSize uint64

		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			newFsSize = suite.mountSize(ctx, cli, userVolumeMountPoint)

			assert.Greater(collect, newFsSize, fsSize)
		}, time.Minute, time.Second)

		suite.T().Logf("user volume filesystem size after resize: %s", humanize.Bytes(newFsSize))
	})
}

func (suite *DiskResizeSuite) growDiskAndWatch(ctx context.Context, cli *client.Client, nodeName string, diskDevice string, diskIdx int, qemuDiskID string, growthSize uint64) {
	diskWatchCh := make(chan safe.WrappedStateEvent[*block.Disk])

	suite.Require().NoError(safe.StateWatch(ctx, cli.COSI, block.NewDisk(block.NamespaceName, diskDevice).Metadata(), diskWatchCh))

	var (
		disk *block.Disk
		err  error
	)

	select {
	case ev := <-diskWatchCh:
		suite.Require().NoError(ev.Error())
		suite.Assert().Equal(state.Created, ev.Type())

		disk, err = ev.Resource()
		suite.Require().NoError(err)
	case <-time.After(time.Second):
		suite.Fail("timeout waiting for watch event")
	}

	diskPreSize := disk.TypedSpec().Size

	suite.T().Logf("disk %s size before resize: %s", disk.Metadata().ID(), humanize.Bytes(diskPreSize))

	suite.growQEMUDisk(ctx, nodeName, diskIdx, qemuDiskID, growthSize)

	select {
	case ev := <-diskWatchCh:
		suite.Require().NoError(ev.Error())
		suite.Assert().Equal(state.Updated, ev.Type())

		disk, err = ev.Resource()
		suite.Require().NoError(err)
	case <-time.After(time.Second):
		suite.Fail("timeout waiting for watch event")
	}

	suite.Assert().Equal(diskPreSize+growthSize, disk.TypedSpec().Size)

	suite.T().Logf("disk %s size after resize: %s", disk.Metadata().ID(), humanize.Bytes(disk.TypedSpec().Size))
}

func (suite *DiskResizeSuite) growQEMUDisk(ctx context.Context, nodeName string, diskIdx int, qemuDiskID string, growthSize uint64) {
	suite.T().Logf("growing disk %d (%s) by %s", diskIdx, qemuDiskID, humanize.Bytes(growthSize))

	// grow the disk
	statePath, err := suite.Cluster.StatePath()
	suite.Require().NoError(err)

	diskPath := filepath.Join(statePath, fmt.Sprintf("%s-%d.disk", nodeName, diskIdx))

	disk, err := os.OpenFile(diskPath, os.O_RDWR, 0)
	suite.Require().NoError(err)

	st, err := disk.Stat()
	suite.Require().NoError(err)

	newSize := st.Size() + int64(growthSize)

	suite.Require().NoError(disk.Truncate(newSize))

	suite.Require().NoError(disk.Close())

	suite.sendMonitorCommand(ctx, nodeName, fmt.Sprintf("block_resize %s %dB", qemuDiskID, newSize))
}

func (suite *DiskResizeSuite) mountSize(ctx context.Context, cli *client.Client, mountPoint string) uint64 {
	resp, err := cli.MachineStorageClient.Statfs(ctx, &machine.StorageServiceStatfsRequest{
		Path: mountPoint,
	})
	suite.Require().NoError(err)

	return resp.GetSize()
}

func init() {
	allSuites = append(
		allSuites,
		&DiskResizeSuite{track: 3},
	)
}
