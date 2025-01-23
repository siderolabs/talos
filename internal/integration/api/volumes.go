// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
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

	for dv := range volumes.All() {
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

			for disk := range disks.All() {
				if disk.TypedSpec().Readonly {
					continue
				}

				if !disk.TypedSpec().CDROM {
					suite.Assert().NotEmpty(disk.TypedSpec().Size, "disk: %s", disk.Metadata().ID())
				}

				suite.Assert().NotEmpty(disk.TypedSpec().Symlinks, "disk: %s", disk.Metadata().ID())
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

				if strings.HasPrefix(disk.Metadata().ID(), "dm-") {
					// devicemapper disks should have secondaries
					suite.Assert().NotEmpty(disk.TypedSpec().SecondaryDisks, "disk: %s", disk.Metadata().ID())

					suite.T().Logf("disk: %s secondaries: %v", disk.Metadata().ID(), disk.TypedSpec().SecondaryDisks)
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

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	nodeName := k8sNode.Name

	suite.T().Logf("creating LVM volume group on node %s/%s", node, nodeName)

	userDisks := suite.UserDisks(suite.ctx, node)

	if len(userDisks) < 2 {
		suite.T().Skipf("skipping test, not enough user disks available on node %s/%s: %q", node, nodeName, userDisks)
	}

	userDisksJoined := strings.Join(userDisks[:2], " ")

	podDef, err := suite.NewPrivilegedPod("pv-create")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

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
		suite.T().Logf("removing LVM volumes %s/%s", node, nodeName)

		deletePodDef, err := suite.NewPrivilegedPod("pv-destroy")
		suite.Require().NoError(err)

		deletePodDef = deletePodDef.WithNodeName(nodeName)

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

	suite.T().Logf("rebooting node %s/%s", node, nodeName)

	// now we want to reboot the node and make sure the array is still mounted
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
		suite.CleanupFailedPods,
	)

	suite.T().Logf("verifying LVM activation %s/%s", node, nodeName)

	suite.Require().Eventually(func() bool {
		return suite.lvmVolumeExists(node)
	}, 5*time.Second, 1*time.Second, "LVM volume group was not activated after reboot")
}

func (suite *VolumesSuite) lvmVolumeExists(node string) bool {
	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	var lvmVolumeCount int

	for disk := range disks.All() {
		if strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/dm") {
			lvmVolumeCount++
		}
	}

	// we test with creating a volume group with two logical volumes
	// one mirrored and one not, so we expect to see at least 6 volumes
	return lvmVolumeCount >= 6
}

// TestSymlinks that Talos can update disk symlinks on the fly.
func (suite *VolumesSuite) TestSymlinks() {
	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping test for non-qemu provisioner")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	nodeName := k8sNode.Name

	userDisks := suite.UserDisks(suite.ctx, node)

	if len(userDisks) < 1 {
		suite.T().Skipf("skipping test, not enough user disks available on node %s/%s: %q", node, nodeName, userDisks)
	}

	userDisk := userDisks[0]
	userDiskName := filepath.Base(userDisk)

	suite.T().Logf("performing a symlink test %s on %s/%s", userDisk, node, nodeName)

	podDef, err := suite.NewPrivilegedPod("xfs-format")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	fsUUID := uuid.New().String()

	_, _, err = podDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- mkfs.xfs -m uuid=%s %s", fsUUID, userDisk),
	)
	suite.Require().NoError(err)

	expectedSymlink := "/dev/disk/by-uuid/" + fsUUID

	// Talos should report a symlink to the disk via FS UUID
	_, err = suite.Client.COSI.WatchFor(client.WithNode(suite.ctx, node), block.NewDisk(block.NamespaceName, userDiskName).Metadata(),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			disk, ok := r.(*block.Disk)
			if !ok {
				return false, fmt.Errorf("unexpected resource type: %T", r)
			}

			return slices.Index(disk.TypedSpec().Symlinks, expectedSymlink) != -1, nil
		}),
	)
	suite.Require().NoError(err)

	suite.T().Logf("wiping user disk %s on %s/%s", userDisk, node, nodeName)

	suite.Require().NoError(suite.Client.BlockDeviceWipe(client.WithNode(suite.ctx, node), &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: userDiskName,
				Method: storage.BlockDeviceWipeDescriptor_FAST,
			},
		},
	}))

	// Talos should remove a symlink to the disk
	_, err = suite.Client.COSI.WatchFor(client.WithNode(suite.ctx, node), block.NewDisk(block.NamespaceName, userDiskName).Metadata(),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			disk, ok := r.(*block.Disk)
			if !ok {
				return false, fmt.Errorf("unexpected resource type: %T", r)
			}

			return slices.Index(disk.TypedSpec().Symlinks, expectedSymlink) == -1, nil
		}),
	)
	suite.Require().NoError(err)
}

func init() {
	allSuites = append(allSuites, new(VolumesSuite))
}
