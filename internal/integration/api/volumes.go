// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
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
	if suite.SelinuxEnforcing {
		suite.T().Skip("skipping tests with nsenter in SELinux enforcing mode")
	}

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

	if len(userDisks) < 2 {
		suite.T().Skipf("skipping test, not enough user disks available on node %s/%s: %q", node, nodeName, userDisks)
	}

	userDisksJoined := strings.Join(userDisks[:2], " ")

	suite.T().Logf("creating LVM volume group on node %s/%s with disks %s", node, nodeName, userDisksJoined)

	podDef, err := suite.NewPrivilegedPod("pv-create")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	stdout, _, err := podDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- vgcreate --nolocking vg0 %s", userDisksJoined),
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
			"nsenter --mount=/proc/1/ns/mnt -- vgremove --nolocking --yes vg0",
		); err != nil {
			suite.T().Logf("failed to remove pv vg0: %v", err)
		}

		if _, _, err := deletePodDef.Exec(
			suite.ctx,
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- pvremove --nolocking --yes %s", userDisksJoined),
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
		return suite.lvmVolumeExists(node, []string{"lv0", "lv1"})
	}, 5*time.Second, 1*time.Second, "LVM volume group was not activated after reboot")
}

func (suite *VolumesSuite) lvmVolumeExists(node string, expectedVolumes []string) bool {
	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	foundVolumes := xslices.ToSet(expectedVolumes)

	// device-mapper volumes will have udevd-created symlinks which contain volume name
	for disk := range disks.All() {
		if strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/dm") {
			for _, volumeName := range expectedVolumes {
				for _, symlink := range disk.TypedSpec().Symlinks {
					if strings.Contains(symlink, volumeName) {
						foundVolumes[volumeName] = struct{}{}

						suite.T().Logf("found LVM volume %s as disk %s with symlink %s", volumeName, disk.Metadata().ID(), symlink)
					}
				}
			}
		}
	}

	return len(foundVolumes) == len(expectedVolumes)
}

// TestSymlinks that Talos can update disk symlinks on the fly.
func (suite *VolumesSuite) TestSymlinks() {
	if suite.SelinuxEnforcing {
		suite.T().Skip("skipping tests with nsenter in SELinux enforcing mode")
	}

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

	suite.T().Logf("performing a symlink test %s on %s/%s with disk %s", userDisk, node, nodeName, userDiskName)

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

// TestUserVolumesStatus verifies that existing user volumes were provisioned successfully.
func (suite *VolumesSuite) TestUserVolumesStatus() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			userVolumeIDs := rtestutils.ResourceIDs[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, state.WithLabelQuery(resource.LabelExists(block.UserVolumeLabel)))

			// check that the volumes are ready
			rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI,
				userVolumeIDs,
				func(vs *block.VolumeStatus, asrt *assert.Assertions) {
					asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
				},
			)

			if len(userVolumeIDs) > 0 {
				suite.T().Logf("found %d user volumes", len(userVolumeIDs))
			}

			// check that the volumes are mounted
			rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI,
				userVolumeIDs,
				func(vs *block.MountStatus, _ *assert.Assertions) {},
			)
		})
	}
}

// TestVolumesStatus verifies that all volumes are either ready or missing.
func (suite *VolumesSuite) TestVolumesStatus() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			rtestutils.AssertAll(ctx, suite.T(), suite.Client.COSI,
				func(vs *block.VolumeStatus, asrt *assert.Assertions) {
					asrt.Contains([]block.VolumePhase{block.VolumePhaseReady, block.VolumePhaseMissing}, vs.TypedSpec().Phase)
				},
			)
		})
	}
}

// TestUserVolumesPartition performs a series of operations on user volumes (partition type): creating, destroying, verifying, etc.
func (suite *VolumesSuite) TestUserVolumesPartition() {
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

	suite.T().Logf("verifying user volumes on node %s/%s with disk %s", node, nodeName, userDisks[0])

	ctx := client.WithNode(suite.ctx, node)

	disk, err := safe.StateGetByID[*block.Disk](ctx, suite.Client.COSI, filepath.Base(userDisks[0]))
	suite.Require().NoError(err)

	volumeName := fmt.Sprintf("%04x", rand.Int31()) + "-"

	const numVolumes = 3

	volumeIDs := make([]string, numVolumes)

	for i := range numVolumes {
		volumeIDs[i] = volumeName + strconv.Itoa(i)
	}

	userVolumeIDs := xslices.Map(volumeIDs, func(volumeID string) string { return constants.UserVolumePrefix + volumeID })

	configDocs := xslices.Map(volumeIDs, func(volumeID string) any {
		doc := blockcfg.NewUserVolumeConfigV1Alpha1()
		doc.MetaName = volumeID
		doc.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
			cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0]), celenv.DiskLocator()),
		)
		doc.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
		doc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("1GiB")

		return doc
	})

	// create user volumes
	suite.PatchMachineConfig(ctx, configDocs...)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// check that the volumes are mounted
	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.MountStatus, _ *assert.Assertions) {})

	// create a pod using user volumes
	podDef, err := suite.NewPod("user-volume-test")
	suite.Require().NoError(err)

	// using subdirectory here to test that the hostPath mount is properly propagated into the kubelet
	podDef = podDef.WithNodeName(nodeName).
		WithNamespace("kube-system").
		WithHostVolumeMount(filepath.Join(constants.UserVolumeMountPoint, volumeIDs[0], "data"), "/mnt/data")

	suite.Require().NoError(podDef.Create(suite.ctx, 1*time.Minute))

	_, _, err = podDef.Exec(suite.ctx, "mkdir -p /mnt/data/test")
	suite.Require().NoError(err)

	suite.Require().NoError(podDef.Delete(suite.ctx))

	// verify that directory exists
	expectedPath := filepath.Join(constants.UserVolumeMountPoint, volumeIDs[0], "data", "test")

	stream, err := suite.Client.LS(ctx, &machineapi.ListRequest{
		Root:  expectedPath,
		Types: []machineapi.ListRequest_Type{machineapi.ListRequest_DIRECTORY},
	})

	suite.Require().NoError(err)

	suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, _ string, _ bool) error {
		suite.T().Logf("found %s on node %s", info.Name, node)
		suite.Require().Equal(expectedPath, info.Name, "expected %s to exist", expectedPath)

		return nil
	}))

	// now, remove one of the volumes, wipe the partition and re-create the volume
	vs, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, suite.Client.COSI, userVolumeIDs[0])
	suite.Require().NoError(err)

	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeIDs[0])

	rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, userVolumeIDs[0])

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		// a little retry loop, as the device might be considered busy for a little while after unmounting
		asrt := assert.New(collect)

		asrt.NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
			Devices: []*storage.BlockDeviceWipeDescriptor{
				{
					Device: filepath.Base(vs.TypedSpec().Location),
					Method: storage.BlockDeviceWipeDescriptor_FAST,
				},
			},
		}))
	}, time.Minute, time.Second, "failed to wipe partition %s", vs.TypedSpec().Location)

	// wait for the discovered volume to disappear
	rtestutils.AssertNoResource[*block.DiscoveredVolume](ctx, suite.T(), suite.Client.COSI, filepath.Base(vs.TypedSpec().Location))

	// re-create the volume with project quota support
	configDocs[0].(*blockcfg.UserVolumeConfigV1Alpha1).FilesystemSpec.ProjectQuotaSupportConfig = pointer.To(true)
	suite.PatchMachineConfig(ctx, configDocs[0])

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.MountStatus, asrt *assert.Assertions) {
			if vs.Metadata().ID() == userVolumeIDs[0] {
				// check that the project quota support is enabled
				asrt.True(vs.TypedSpec().ProjectQuotaSupport, "project quota support should be enabled for %s", vs.Metadata().ID())
			} else {
				// check that the project quota support is disabled
				asrt.False(vs.TypedSpec().ProjectQuotaSupport, "project quota support should be disabled for %s", vs.Metadata().ID())
			}
		})

	// clean up
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeIDs...)

	for _, userVolumeID := range userVolumeIDs {
		rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, userVolumeID)
	}

	suite.Require().NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: filepath.Base(userDisks[0]),
				Method: storage.BlockDeviceWipeDescriptor_FAST,
			},
		},
	}))

	// wait for the discovered volume reflect wiped status
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, filepath.Base(userDisks[0]),
		func(dv *block.DiscoveredVolume, asrt *assert.Assertions) {
			asrt.Empty(dv.TypedSpec().Name, "expected discovered volume %s to be wiped", dv.Metadata().ID())
		})
}

// TestUserVolumesDisk performs a series of operations on user volumes (disk type): creating, destroying, verifying, etc.
func (suite *VolumesSuite) TestUserVolumesDisk() {
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

	suite.T().Logf("verifying user volumes on node %s/%s with disk %s", node, nodeName, userDisks[0])

	ctx := client.WithNode(suite.ctx, node)

	disk, err := safe.StateGetByID[*block.Disk](ctx, suite.Client.COSI, filepath.Base(userDisks[0]))
	suite.Require().NoError(err)

	volumeName := fmt.Sprintf("%04x", rand.Int31()) + "-"

	const numVolumes = 1

	volumeIDs := make([]string, numVolumes)

	for i := range numVolumes {
		volumeIDs[i] = volumeName + strconv.Itoa(i)
	}

	userVolumeIDs := xslices.Map(volumeIDs, func(volumeID string) string { return constants.UserVolumePrefix + volumeID })

	configDocs := xslices.Map(volumeIDs, func(volumeID string) any {
		doc := blockcfg.NewUserVolumeConfigV1Alpha1()
		doc.MetaName = volumeID
		doc.VolumeType = pointer.To(block.VolumeTypeDisk)
		doc.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
			cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0]), celenv.DiskLocator()),
		)

		return doc
	})

	// create user volumes
	suite.PatchMachineConfig(ctx, configDocs...)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// check that the volumes are mounted
	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.MountStatus, _ *assert.Assertions) {})

	// create a pod using user volumes
	podDef, err := suite.NewPod("user-volume-test")
	suite.Require().NoError(err)

	// using subdirectory here to test that the hostPath mount is properly propagated into the kubelet
	podDef = podDef.WithNodeName(nodeName).
		WithNamespace("kube-system").
		WithHostVolumeMount(filepath.Join(constants.UserVolumeMountPoint, volumeIDs[0], "data"), "/mnt/data")

	suite.Require().NoError(podDef.Create(suite.ctx, 1*time.Minute))

	_, _, err = podDef.Exec(suite.ctx, "mkdir -p /mnt/data/test")
	suite.Require().NoError(err)

	suite.Require().NoError(podDef.Delete(suite.ctx))

	// verify that directory exists
	expectedPath := filepath.Join(constants.UserVolumeMountPoint, volumeIDs[0], "data", "test")

	stream, err := suite.Client.LS(ctx, &machineapi.ListRequest{
		Root:  expectedPath,
		Types: []machineapi.ListRequest_Type{machineapi.ListRequest_DIRECTORY},
	})

	suite.Require().NoError(err)

	suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, _ string, _ bool) error {
		suite.T().Logf("found %s on node %s", info.Name, node)
		suite.Require().Equal(expectedPath, info.Name, "expected %s to exist", expectedPath)

		return nil
	}))

	// verify that volume labels are set properly
	expectedLabels := xslices.ToSet(userVolumeIDs)

	dvs, err := safe.StateListAll[*block.DiscoveredVolume](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	for dv := range dvs.All() {
		delete(expectedLabels, dv.TypedSpec().PartitionLabel)
	}

	suite.Require().Empty(expectedLabels, "expected labels %v to be set on discovered volumes", expectedLabels)

	// now, remove one of the volumes, wipe the partition and re-create the volume
	vs, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, suite.Client.COSI, userVolumeIDs[0])
	suite.Require().NoError(err)

	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeIDs[0])

	rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, userVolumeIDs[0])

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		// a little retry loop, as the device might be considered busy for a little while after unmounting
		asrt := assert.New(collect)

		asrt.NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
			Devices: []*storage.BlockDeviceWipeDescriptor{
				{
					Device:        filepath.Base(vs.TypedSpec().Location),
					Method:        storage.BlockDeviceWipeDescriptor_FAST,
					DropPartition: true,
				},
			},
		}))
	}, time.Minute, time.Second, "failed to wipe partition %s", vs.TypedSpec().Location)

	// wait for the discovered volume to disappear
	rtestutils.AssertNoResource[*block.DiscoveredVolume](ctx, suite.T(), suite.Client.COSI, filepath.Base(vs.TypedSpec().Location))

	// re-create the volume with project quota support
	configDocs[0].(*blockcfg.UserVolumeConfigV1Alpha1).FilesystemSpec.ProjectQuotaSupportConfig = pointer.To(true)
	suite.PatchMachineConfig(ctx, configDocs[0])

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.MountStatus, asrt *assert.Assertions) {
			if vs.Metadata().ID() == userVolumeIDs[0] {
				// check that the project quota support is enabled
				asrt.True(vs.TypedSpec().ProjectQuotaSupport, "project quota support should be enabled for %s", vs.Metadata().ID())
			} else {
				// check that the project quota support is disabled
				asrt.False(vs.TypedSpec().ProjectQuotaSupport, "project quota support should be disabled for %s", vs.Metadata().ID())
			}
		})

	// clean up
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeIDs...)

	for _, userVolumeID := range userVolumeIDs {
		rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, userVolumeID)
	}

	suite.Require().NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: filepath.Base(userDisks[0]),
				Method: storage.BlockDeviceWipeDescriptor_FAST,
			},
		},
	}))

	// wait for the discovered volume reflect wiped status
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, filepath.Base(userDisks[0]),
		func(dv *block.DiscoveredVolume, asrt *assert.Assertions) {
			asrt.Empty(dv.TypedSpec().Name, "expected discovered volume %s to be wiped", dv.Metadata().ID())
		})
}

// TestUserVolumesDirectory performs a series of operations on user volumes (directory type): creating, destroying, verifying, etc.
func (suite *VolumesSuite) TestUserVolumesDirectory() {
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

	ctx := client.WithNode(suite.ctx, node)

	volumeName := fmt.Sprintf("%04x", rand.Int31()) + "-"

	const numVolumes = 3

	volumeIDs := make([]string, numVolumes)

	for i := range numVolumes {
		volumeIDs[i] = volumeName + strconv.Itoa(i)
	}

	userVolumeIDs := xslices.Map(volumeIDs, func(volumeID string) string { return constants.UserVolumePrefix + volumeID })

	configDocs := xslices.Map(volumeIDs, func(volumeID string) any {
		doc := blockcfg.NewUserVolumeConfigV1Alpha1()
		doc.MetaName = volumeID
		doc.VolumeType = pointer.To(block.VolumeTypeDirectory)

		return doc
	})

	// create user volumes
	suite.PatchMachineConfig(ctx, configDocs...)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// check that the volumes are mounted
	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.MountStatus, _ *assert.Assertions) {})

	// create a pod using user volumes
	podDef, err := suite.NewPod("user-volume-test")
	suite.Require().NoError(err)

	// using subdirectory here to test that the hostPath mount is properly propagated into the kubelet
	podDef = podDef.WithNodeName(nodeName).
		WithNamespace("kube-system").
		WithHostVolumeMount(filepath.Join(constants.UserVolumeMountPoint, volumeIDs[0], "data"), "/mnt/data")

	suite.Require().NoError(podDef.Create(suite.ctx, 1*time.Minute))

	_, _, err = podDef.Exec(suite.ctx, "mkdir -p /mnt/data/test")
	suite.Require().NoError(err)

	suite.Require().NoError(podDef.Delete(suite.ctx))

	// verify that directory exists
	expectedPath := filepath.Join(constants.UserVolumeMountPoint, volumeIDs[0], "data", "test")

	stream, err := suite.Client.LS(ctx, &machineapi.ListRequest{
		Root:  expectedPath,
		Types: []machineapi.ListRequest_Type{machineapi.ListRequest_DIRECTORY},
	})

	suite.Require().NoError(err)

	suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, _ string, _ bool) error {
		suite.T().Logf("found %s on node %s", info.Name, node)
		suite.Require().Equal(expectedPath, info.Name, "expected %s to exist", expectedPath)

		return nil
	}))

	// now, remove one of the volumes and re-create the volume
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeIDs[0])

	rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, userVolumeIDs[0])

	// re-create the volume
	suite.PatchMachineConfig(ctx, configDocs[0])

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, userVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// clean up
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeIDs...)

	for _, userVolumeID := range userVolumeIDs {
		rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, userVolumeID)
	}
}

// TestRawVolumes performs a series of operations on raw volumes: creating, destroying, etc.
func (suite *VolumesSuite) TestRawVolumes() {
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

	suite.T().Logf("verifying raw volumes on node %s/%s with disk %s", node, nodeName, userDisks[0])

	ctx := client.WithNode(suite.ctx, node)

	disk, err := safe.StateGetByID[*block.Disk](ctx, suite.Client.COSI, filepath.Base(userDisks[0]))
	suite.Require().NoError(err)

	volumeName := fmt.Sprintf("%04x", rand.Int31()) + "-"

	const numVolumes = 2

	volumeIDs := make([]string, numVolumes)

	for i := range numVolumes {
		volumeIDs[i] = volumeName + strconv.Itoa(i)
	}

	rawVolumeIDs := xslices.Map(volumeIDs, func(volumeID string) string { return constants.RawVolumePrefix + volumeID })

	configDocs := xslices.Map(volumeIDs, func(volumeID string) any {
		doc := blockcfg.NewRawVolumeConfigV1Alpha1()
		doc.MetaName = volumeID
		doc.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
			cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0]), celenv.DiskLocator()),
		)
		doc.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
		doc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("500MiB")

		return doc
	})

	// create raw volumes
	suite.PatchMachineConfig(ctx, configDocs...)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, rawVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// verify that volume labels are set properly
	for _, rawVolumeID := range rawVolumeIDs {
		vs, err := safe.StateGetByID[*block.VolumeStatus](ctx, suite.Client.COSI, rawVolumeID)
		suite.Require().NoError(err)

		rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, filepath.Base(vs.TypedSpec().Location),
			func(dv *block.DiscoveredVolume, asrt *assert.Assertions) {
				asrt.Equal(vs.Metadata().ID(), dv.TypedSpec().PartitionLabel, "expected discovered volume %s to have label %s", dv.Metadata().ID(), vs.Metadata().ID())
			})
	}

	// now, remove one of the volumes, wipe the partition and re-create the volume
	vs, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, suite.Client.COSI, rawVolumeIDs[0])
	suite.Require().NoError(err)

	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.RawVolumeConfigKind, volumeIDs[0])

	rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, rawVolumeIDs[0])

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		// a little retry loop, as the device might be considered busy for a little while after unmounting
		asrt := assert.New(collect)

		asrt.NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
			Devices: []*storage.BlockDeviceWipeDescriptor{
				{
					Device:        filepath.Base(vs.TypedSpec().Location),
					Method:        storage.BlockDeviceWipeDescriptor_FAST,
					DropPartition: true,
				},
			},
		}))
	}, time.Minute, time.Second, "failed to wipe partition %s", vs.TypedSpec().Location)

	// wait for the discovered volume to disappear
	rtestutils.AssertNoResource[*block.DiscoveredVolume](ctx, suite.T(), suite.Client.COSI, filepath.Base(vs.TypedSpec().Location))

	// re-create the volume
	suite.PatchMachineConfig(ctx, configDocs[0])

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, rawVolumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// clean up
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.RawVolumeConfigKind, volumeIDs...)

	for _, rawVolumeID := range rawVolumeIDs {
		rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, rawVolumeID)
	}

	suite.Require().NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: filepath.Base(userDisks[0]),
				Method: storage.BlockDeviceWipeDescriptor_FAST,
			},
		},
	}))

	// wait for the discovered volume reflect wiped status
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, filepath.Base(userDisks[0]),
		func(dv *block.DiscoveredVolume, asrt *assert.Assertions) {
			asrt.Empty(dv.TypedSpec().Name, "expected discovered volume %s to be wiped", dv.Metadata().ID())
		})
}

// TestExistingVolumes performs a series of operations on existing volumes: mount/unmount, etc.
func (suite *VolumesSuite) TestExistingVolumes() {
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

	suite.T().Logf("verifying existing volumes on node %s/%s with disk %s", node, nodeName, userDisks[0])

	ctx := client.WithNode(suite.ctx, node)

	disk, err := safe.StateGetByID[*block.Disk](ctx, suite.Client.COSI, filepath.Base(userDisks[0]))
	suite.Require().NoError(err)

	volumeID := fmt.Sprintf("%04x", rand.Int31())
	existingVolumeID := constants.ExistingVolumePrefix + volumeID

	// first, create a user volume config to get the volume created
	userVolumeID := constants.UserVolumePrefix + volumeID

	userVolumeDoc := blockcfg.NewUserVolumeConfigV1Alpha1()
	userVolumeDoc.MetaName = volumeID
	userVolumeDoc.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0]), celenv.DiskLocator()),
	)
	userVolumeDoc.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
	userVolumeDoc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("1GiB")

	// create user volumes
	suite.PatchMachineConfig(ctx, userVolumeDoc)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{userVolumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// now destroy a user volume
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeID)

	// wait for the user volume to be removed
	rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, userVolumeID)

	// now, recreate the volume as an existing volume
	existingVolumeDoc := blockcfg.NewExistingVolumeConfigV1Alpha1()
	existingVolumeDoc.MetaName = volumeID
	existingVolumeDoc.VolumeDiscoverySpec.VolumeSelectorConfig.Match = cel.MustExpression(
		cel.ParseBooleanExpression(
			fmt.Sprintf("volume.partition_label == '%s' && '%s' in disk.symlinks", userVolumeID, disk.TypedSpec().Symlinks[0]),
			celenv.VolumeLocator(),
		),
	)

	// create existing volume
	suite.PatchMachineConfig(ctx, existingVolumeDoc)

	// wait for the existing volume to be discovered
	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{existingVolumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
			asrt.Equal(userDisks[0], vs.TypedSpec().ParentLocation)
		},
	)

	// check that the volume is mounted
	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{existingVolumeID},
		func(vs *block.MountStatus, asrt *assert.Assertions) {
			asrt.False(vs.TypedSpec().ReadOnly)
		})

	// create a pod using existing volumes
	podDef, err := suite.NewPod("existing-volume-test")
	suite.Require().NoError(err)

	// using subdirectory here to test that the hostPath mount is properly propagated into the kubelet
	podDef = podDef.WithNodeName(nodeName).
		WithNamespace("kube-system").
		WithHostVolumeMount(filepath.Join(constants.UserVolumeMountPoint, volumeID, "data"), "/mnt/data")

	suite.Require().NoError(podDef.Create(suite.ctx, 1*time.Minute))

	_, _, err = podDef.Exec(suite.ctx, "mkdir -p /mnt/data/test")
	suite.Require().NoError(err)

	suite.Require().NoError(podDef.Delete(suite.ctx))

	// verify that directory exists
	expectedPath := filepath.Join(constants.UserVolumeMountPoint, volumeID, "data", "test")

	stream, err := suite.Client.LS(ctx, &machineapi.ListRequest{
		Root:  expectedPath,
		Types: []machineapi.ListRequest_Type{machineapi.ListRequest_DIRECTORY},
	})

	suite.Require().NoError(err)

	suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, _ string, _ bool) error {
		suite.T().Logf("found %s on node %s", info.Name, node)
		suite.Require().Equal(expectedPath, info.Name, "expected %s to exist", expectedPath)

		return nil
	}))

	// now, re-mount the existing volume as read-only
	existingVolumeDoc.MountSpec.MountReadOnly = pointer.To(true)
	suite.PatchMachineConfig(ctx, existingVolumeDoc)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{existingVolumeID},
		func(vs *block.MountStatus, asrt *assert.Assertions) {
			asrt.True(vs.TypedSpec().ReadOnly)
		})

	// clean up
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.ExistingVolumeConfigKind, volumeID)

	rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, existingVolumeID)

	suite.Require().NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: filepath.Base(userDisks[0]),
				Method: storage.BlockDeviceWipeDescriptor_FAST,
			},
		},
	}))

	// wait for the discovered volume reflect wiped status
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, filepath.Base(userDisks[0]),
		func(dv *block.DiscoveredVolume, asrt *assert.Assertions) {
			asrt.Empty(dv.TypedSpec().Name, "expected discovered volume %s to be wiped", dv.Metadata().ID())
		})
}

// TestSwapStatus verifies that all swap volumes are successfully enabled.
func (suite *VolumesSuite) TestSwapStatus() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			swapVolumes, err := safe.StateListAll[*block.VolumeConfig](ctx, suite.Client.COSI, state.WithLabelQuery(resource.LabelExists(block.SwapVolumeLabel)))
			suite.Require().NoError(err)

			if swapVolumes.Len() == 0 {
				suite.T().Skipf("skipping test, no swap volumes found on node %s", node)
			}

			rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI,
				xslices.Map(slices.Collect(swapVolumes.All()), func(sv *block.VolumeConfig) string {
					return sv.Metadata().ID()
				}),
				func(vs *block.VolumeStatus, asrt *assert.Assertions) {
					asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
				},
			)

			swapVolumesStatus, err := safe.StateListAll[*block.VolumeStatus](ctx, suite.Client.COSI, state.WithLabelQuery(resource.LabelExists(block.SwapVolumeLabel)))
			suite.Require().NoError(err)

			deviceNames := xslices.Map(slices.Collect(swapVolumesStatus.All()), func(sv *block.VolumeStatus) string {
				return sv.TypedSpec().MountLocation
			})

			rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI,
				deviceNames,
				func(vs *block.SwapStatus, asrt *assert.Assertions) {},
			)

			suite.T().Logf("found swap volumes (%q) on node %s", deviceNames, node)
		})
	}
}

// TestSwapOnOff performs a series of operations on swap volume: creating, destroying, enabling, disabling, etc.
func (suite *VolumesSuite) TestSwapOnOff() {
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

	suite.T().Logf("verifying swap on node %s/%s with disk %s", node, nodeName, userDisks[0])

	ctx := client.WithNode(suite.ctx, node)

	disk, err := safe.StateGetByID[*block.Disk](ctx, suite.Client.COSI, filepath.Base(userDisks[0]))
	suite.Require().NoError(err)

	volumeName := fmt.Sprintf("%04x", rand.Int31())

	doc := blockcfg.NewSwapVolumeConfigV1Alpha1()
	doc.MetaName = volumeName
	doc.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0]), celenv.DiskLocator()),
	)
	doc.EncryptionSpec = blockcfg.EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []blockcfg.EncryptionKey{
			{
				KeySlot: 0,
				KeyStatic: &blockcfg.EncryptionKeyStatic{
					KeyData: "secretswap",
				},
			},
		},
	}
	doc.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
	doc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("500MiB")

	// create user volumes
	suite.PatchMachineConfig(ctx, doc)

	swapVolumeID := constants.SwapVolumePrefix + doc.MetaName

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []string{swapVolumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	// check that the volumes are mounted
	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []string{swapVolumeID},
		func(vs *block.MountStatus, _ *assert.Assertions) {})

	// check that the swap is enabled
	volumeStatus, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, suite.Client.COSI, swapVolumeID)
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []string{volumeStatus.TypedSpec().MountLocation},
		func(vs *block.SwapStatus, asrt *assert.Assertions) {})

	// clean up
	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.SwapVolumeConfigKind, volumeName)

	rtestutils.AssertNoResource[*block.VolumeStatus](ctx, suite.T(), suite.Client.COSI, swapVolumeID)
	rtestutils.AssertNoResource[*block.SwapStatus](ctx, suite.T(), suite.Client.COSI, volumeStatus.TypedSpec().MountLocation)

	suite.Require().NoError(suite.Client.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: filepath.Base(userDisks[0]),
				Method: storage.BlockDeviceWipeDescriptor_FAST,
			},
		},
	}))
}

// TestZswapStatus verifies that all zswap-enabled machines have zswap running.
func (suite *VolumesSuite) TestZswapStatus() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			cfg, err := suite.ReadConfigFromNode(ctx)
			suite.Require().NoError(err)

			if cfg.ZswapConfig() == nil {
				suite.T().Skipf("skipping test, zswap is not enabled on node %s", node)
			}

			rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI,
				block.ZswapStatusID,
				func(vs *block.ZswapStatus, asrt *assert.Assertions) {
					suite.T().Logf("zswap total size %s, stored pages %d", vs.TypedSpec().TotalSizeHuman, vs.TypedSpec().StoredPages)
				},
			)
		})
	}
}

func init() {
	allSuites = append(allSuites, new(VolumesSuite))
}
