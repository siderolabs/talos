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

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/client"
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

	suite.setupCluster(clusterOptions{
		ClusterName: "disk-resize",

		ControlplaneNodes: 1, // just a single node

		SourceKernelPath:     helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath:  helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: sourceInstallerImage,
		SourceVersion:        DefaultSettings.CurrentVersion,
		SourceK8sVersion:     constants.DefaultKubernetesVersion,
	})

	cli, err := suite.clusterAccess.Client()
	suite.Require().NoError(err)

	ctx := client.WithNode(suite.ctx, suite.clusterAccess.Nodes()[0].IPs[0].String())

	// verify current settings of the partition
	ephemeralStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, cli.COSI, constants.EphemeralPartitionLabel)
	suite.Require().NoError(err)

	preSize := ephemeralStatus.TypedSpec().Size

	suite.T().Logf("EPHEMERAL partition size before resize: %s", humanize.Bytes(preSize))

	fsSize := suite.mountSize(ctx, cli, "/var")

	suite.T().Logf("EPHEMERAL partition filesystem size before resize: %s", humanize.Bytes(fsSize))

	diskWatchCh := make(chan safe.WrappedStateEvent[*block.Disk])

	suite.Require().NoError(safe.StateWatch(ctx, cli.COSI, block.NewDisk(block.NamespaceName, "vda").Metadata(), diskWatchCh))

	var disk *block.Disk

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

	const growthSize = 1 * 1000 * 1000 * 10000 // 1GB

	suite.growPrimaryDisk(ctx, suite.Cluster.Info().Nodes[0].Name, growthSize)

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

func (suite *DiskResizeSuite) growPrimaryDisk(ctx context.Context, nodeName string, growthSize uint64) {
	suite.T().Logf("growing primary disk by %s", humanize.Bytes(growthSize))

	// grow the disk
	statePath, err := suite.Cluster.StatePath()
	suite.Require().NoError(err)

	diskPath := filepath.Join(statePath, nodeName+"-0.disk")

	disk, err := os.OpenFile(diskPath, os.O_RDWR, 0)
	suite.Require().NoError(err)

	st, err := disk.Stat()
	suite.Require().NoError(err)

	newSize := st.Size() + int64(growthSize)

	suite.Require().NoError(disk.Truncate(newSize))

	suite.Require().NoError(disk.Close())

	suite.sendMonitorCommand(ctx, nodeName, fmt.Sprintf("block_resize virtio0 %dB", newSize))
}

func (suite *DiskResizeSuite) mountSize(ctx context.Context, cli *client.Client, mountPoint string) uint64 {
	resp, err := cli.Mounts(ctx)
	suite.Require().NoError(err)

	for _, m := range resp.GetMessages() {
		for _, mount := range m.GetStats() {
			if mount.GetMountedOn() == mountPoint {
				return mount.GetSize()
			}
		}
	}

	suite.FailNow("mountpoint not found", "mountpoint: %s", mountPoint)

	panic("unreachable")
}

func init() {
	allSuites = append(
		allSuites,
		&DiskResizeSuite{track: 3},
	)
}
