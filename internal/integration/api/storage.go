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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// StorageSuite covers the storage.* controllers that observe (rather than
// provision) host state — currently the LVM PV/VG/LV status controllers.
//
// Talos has no first-class LVM provisioning API, so tests in this suite drive
// LVM state from a privileged pod via `nsenter --mount=/proc/1/ns/mnt --` in
// the same style as TestLVMActivation in the VolumesSuite.
type StorageSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName implements the suite.NamedSuite interface.
func (suite *StorageSuite) SuiteName() string {
	return "api.StorageSuite"
}

// SetupTest ...
func (suite *StorageSuite) SetupTest() {
	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *StorageSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestLVMStatus verifies that LVM status controllers populate
// LVMVolumeGroupStatus, LVMPhysicalVolumeStatus and LVMLogicalVolumeStatus
// resources reflecting the VG, PVs and LVs created on a node, and that the
// resources are cleaned up when the underlying LVs disappear.
//
//nolint:gocyclo,cyclop
func (suite *StorageSuite) TestLVMStatus() {
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

	pvDisks := userDisks[:2]
	pvDisksJoined := strings.Join(pvDisks, " ")

	suite.T().Logf("creating LVM volume group on node %s/%s with disks %s", node, nodeName, pvDisksJoined)

	podDef, err := suite.NewPrivilegedPod("lvm-status")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	stdout, _, err := podDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- vgcreate --nolocking vg0 %s", pvDisksJoined),
	)
	suite.Require().NoError(err)
	suite.Require().Contains(stdout, "Volume group \"vg0\" successfully created")

	defer suite.deleteLVMVolumes(node, nodeName, pvDisksJoined)

	stdout, _, err = podDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- lvcreate -n lv0 -L 64M vg0",
	)
	suite.Require().NoError(err)
	suite.Require().Contains(stdout, "Logical volume \"lv0\" created.")

	stdout, _, err = podDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- lvcreate -n lv1 -L 64M vg0",
	)
	suite.Require().NoError(err)
	suite.Require().Contains(stdout, "Logical volume \"lv1\" created.")

	ctx := client.WithNode(suite.ctx, node)

	// Status controllers poll every 30s; allow a generous window for the first
	// reconcile that follows our create commands.
	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	suite.T().Logf("waiting for VG status on %s/%s", node, nodeName)

	suite.Require().Eventually(func() bool {
		vg, err := safe.StateGetByID[*storageres.LVMVolumeGroupStatus](ctx, suite.Client.COSI, "vg0")
		if err != nil {
			if state.IsNotFoundError(err) {
				return false
			}

			suite.T().Logf("unexpected error reading vg status: %v", err)

			return false
		}

		spec := vg.TypedSpec()

		return spec.Name == "vg0" && spec.PVCount == "2" && spec.LVCount == "2" && spec.Size != "" && spec.Size != "0" && spec.UUID != ""
	}, assertTimeout, assertInterval, "VG status not reported")

	suite.T().Logf("waiting for PV status on %s/%s", node, nodeName)

	expectedPVIDs := xslices.ToSet(xslices.Map(pvDisks, func(d string) string {
		return strings.TrimPrefix(strings.ReplaceAll(d, "/", "-"), "-dev-")
	}))

	suite.Require().Eventually(func() bool {
		pvs, err := safe.StateListAll[*storageres.LVMPhysicalVolumeStatus](ctx, suite.Client.COSI)
		if err != nil {
			suite.T().Logf("unexpected error listing pv statuses: %v", err)

			return false
		}

		found := map[string]struct{}{}

		for pv := range pvs.All() {
			id := pv.Metadata().ID()
			if _, ok := expectedPVIDs[id]; !ok {
				continue
			}

			spec := pv.TypedSpec()
			if spec.VGName != "vg0" || spec.Size == "" || spec.Size == "0" || spec.UUID == "" {
				continue
			}

			found[id] = struct{}{}
		}

		return len(found) == len(expectedPVIDs)
	}, assertTimeout, assertInterval, "PV statuses not reported for disks %v", pvDisks)

	suite.T().Logf("waiting for LV status on %s/%s", node, nodeName)

	expectedLVPaths := xslices.ToSet([]string{"/dev/vg0/lv0", "/dev/vg0/lv1"})

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](ctx, suite.Client.COSI)
		if err != nil {
			suite.T().Logf("unexpected error listing lv statuses: %v", err)

			return false
		}

		found := map[string]struct{}{}

		for lv := range lvs.All() {
			spec := lv.TypedSpec()
			if _, ok := expectedLVPaths[spec.Path]; !ok {
				continue
			}

			if spec.VGName != "vg0" || spec.Size == "" || spec.Size == "0" {
				continue
			}

			found[spec.Path] = struct{}{}
		}

		return len(found) == len(expectedLVPaths)
	}, assertTimeout, assertInterval, "LV statuses not reported")

	// Remove LVs explicitly (before the deferred VG cleanup) so we can verify
	// that the LV status controller drops the resources while the pod is still
	// alive and reachable.
	suite.T().Logf("removing LVs and verifying status cleanup on %s/%s", node, nodeName)

	if _, _, err := podDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- lvremove --nolocking --yes vg0/lv0 vg0/lv1",
	); err != nil {
		suite.T().Logf("failed to remove LVs: %v", err)
	}

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](ctx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for lv := range lvs.All() {
			if _, ok := expectedLVPaths[lv.TypedSpec().Path]; ok {
				return false
			}
		}

		return true
	}, assertTimeout, assertInterval, "LV statuses were not cleaned up")
}

// TestLVMActivation verifies that an LVM volume group is reactivated after reboot.
func (suite *StorageSuite) TestLVMActivation() {
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

	defer suite.deleteLVMVolumes(node, nodeName, userDisksJoined)

	suite.T().Logf("rebooting node %s/%s", node, nodeName)

	// reboot and confirm that LVs come back online
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

// deleteLVMVolumes removes the VG and PV labels created by the LVM tests.
func (suite *StorageSuite) deleteLVMVolumes(node, nodeName, userDisksJoined string) {
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
}

// lvmVolumeExists returns true once every LV name in expectedVolumes is visible
// as a /dev/dm-* disk symlink.
func (suite *StorageSuite) lvmVolumeExists(node string, expectedVolumes []string) bool {
	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	foundVolumes := xslices.ToSet(expectedVolumes)

	// device-mapper volumes have udevd-created symlinks containing the LV name
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

func init() {
	allSuites = append(allSuites, new(StorageSuite))
}
