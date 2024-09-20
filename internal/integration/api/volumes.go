// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumesSuite ...
type VolumesSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *VolumesSuite) SuiteName() string {
	return "api.VolumesSuite"
}

// SetupTest ...
func (suite *VolumesSuite) SetupTest() {
	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Minute)
}

// TearDownTest ...
func (suite *VolumesSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestDiscoveredVolumes verifies that standard Talos partitions are discovered.
func (suite *VolumesSuite) TestDiscoveredVolumes() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			suite.testDiscoveredVolumes(node)
		})
	}
}

func (suite *VolumesSuite) testDiscoveredVolumes(node string) {
	ctx := client.WithNode(suite.ctx, node)

	volumes, err := safe.StateListAll[*block.DiscoveredVolume](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	expectedVolumes := map[string]struct {
		Names []string
	}{
		"META": {
			Names: []string{"talosmeta", ""}, // if META was never written, it will not be detected
		},
		"STATE": {
			Names: []string{"xfs"},
		},
		"EPHEMERAL": {
			Names: []string{"xfs", ""},
		},
	}

	for iterator := volumes.Iterator(); iterator.Next(); {
		dv := iterator.Value()

		suite.T().Logf("volume: %s %s %s %s", dv.Metadata().ID(), dv.TypedSpec().Name, dv.TypedSpec().PartitionLabel, dv.TypedSpec().Label)

		partitionLabel := dv.TypedSpec().PartitionLabel
		filesystemLabel := dv.TypedSpec().Label

		// this is encrypted partition, skip it, we should see another device with the actual filesystem
		if dv.TypedSpec().Name == "luks" {
			continue
		}

		// match either by partition or filesystem label
		id := partitionLabel

		expected, ok := expectedVolumes[id]
		if !ok {
			id = filesystemLabel

			expected, ok = expectedVolumes[id]

			if !ok {
				continue
			}
		}

		suite.Assert().Contains(expected.Names, dv.TypedSpec().Name, "node: %s", node)

		delete(expectedVolumes, id)
	}

	suite.Assert().Empty(expectedVolumes, "node: ", node)

	if suite.T().Failed() {
		suite.DumpLogs(suite.ctx, node, "controller-runtime", "block.")
	}
}

// TestSystemDisk verifies that Talos system disk is discovered.
func (suite *VolumesSuite) TestSystemDisk() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			systemDisk, err := safe.StateGetByID[*block.SystemDisk](ctx, suite.Client.COSI, block.SystemDiskID)
			suite.Require().NoError(err)

			suite.Assert().NotEmpty(systemDisk.TypedSpec().DiskID)

			suite.T().Logf("system disk: %s", systemDisk.TypedSpec().DiskID)
		})
	}
}

// TestDisks verifies that Talos discovers disks.
func (suite *VolumesSuite) TestDisks() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
			suite.Require().NoError(err)

			// there should be at least two disks - loop0 for Talos squashfs and a system disk
			suite.Assert().Greater(disks.Len(), 1)

			var diskNames []string

			for iter := disks.Iterator(); iter.Next(); {
				disk := iter.Value()

				if disk.TypedSpec().Readonly {
					continue
				}

				if !disk.TypedSpec().CDROM {
					suite.Assert().NotEmpty(disk.TypedSpec().Size, "disk: %s", disk.Metadata().ID())
				}

				suite.Assert().NotEmpty(disk.TypedSpec().IOSize, "disk: %s", disk.Metadata().ID())
				suite.Assert().NotEmpty(disk.TypedSpec().SectorSize, "disk: %s", disk.Metadata().ID())

				if suite.Cluster != nil {
					// running on our own provider, transport should be always detected
					if disk.TypedSpec().BusPath == "/virtual" {
						suite.Assert().Empty(disk.TypedSpec().Transport, "disk: %s", disk.Metadata().ID())
					} else {
						suite.Assert().NotEmpty(disk.TypedSpec().Transport, "disk: %s", disk.Metadata().ID())
					}
				}

				diskNames = append(diskNames, disk.Metadata().ID())
			}

			suite.T().Logf("disks: %v", diskNames)
		})
	}
}

// TestLVMActivation verifies that LVM volume group is activated after reboot.
func (suite *VolumesSuite) TestLVMActivation() {
	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping test for non-qemu provisioner")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	userDisks, err := suite.UserDisks(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Require().GreaterOrEqual(len(userDisks), 2, "expected at least two user disks to be available")

	userDisksJoined := strings.Join(userDisks[:2], " ")

	podDef, err := suite.NewPrivilegedPod("pv-create")
	suite.Require().NoError(err)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	stdout, _, err := podDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- vgcreate vg0 %s", userDisksJoined),
	)
	suite.Require().NoError(err)

	suite.Require().Contains(stdout, "Volume group \"vg0\" successfully created")

	stdout, _, err = podDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- lvcreate --mirrors=1 --type=raid1 --nosync -n lv0 -L 1G vg0",
	)
	suite.Require().NoError(err)

	suite.Require().Contains(stdout, "Logical volume \"lv0\" created.")

	stdout, _, err = podDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- lvcreate -n lv1 -L 1G vg0",
	)
	suite.Require().NoError(err)

	suite.Require().Contains(stdout, "Logical volume \"lv1\" created.")

	defer func() {
		deletePodDef, err := suite.NewPrivilegedPod("pv-destroy")
		suite.Require().NoError(err)

		suite.Require().NoError(deletePodDef.Create(suite.ctx, 5*time.Minute))

		defer deletePodDef.Delete(suite.ctx) //nolint:errcheck

		if _, _, err := deletePodDef.Exec(
			suite.ctx,
			"nsenter --mount=/proc/1/ns/mnt -- vgremove --yes vg0",
		); err != nil {
			suite.T().Logf("failed to remove pv vg0: %v", err)
		}

		if _, _, err := deletePodDef.Exec(
			suite.ctx,
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- pvremove --yes %s", userDisksJoined),
		); err != nil {
			suite.T().Logf("failed to remove pv backed by volumes %s: %v", userDisksJoined, err)
		}
	}()

	// now we want to reboot the node and make sure the array is still mounted
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
	)

	suite.Require().True(suite.lvmVolumeExists(), "LVM volume group was not activated after reboot")
}

func (suite *VolumesSuite) lvmVolumeExists() bool {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	var lvmVolumeCount int

	for iterator := disks.Iterator(); iterator.Next(); {
		if strings.HasPrefix(iterator.Value().TypedSpec().DevPath, "/dev/dm") {
			lvmVolumeCount++
		}
	}

	// we test with creating a volume group with two logical volumes
	// one mirrored and one not, so we expect to see 6 volumes
	return lvmVolumeCount == 6
}

func init() {
	allSuites = append(allSuites, new(VolumesSuite))
}
