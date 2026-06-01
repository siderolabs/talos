// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	storageapi "github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// StorageSuite covers the declarative LVM provisioning flow.
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

const vgName = "vg0"

// newVGConfig builds a doc whose selector matches the supplied disks.
func newVGConfig(pvDisks []string) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
	doc := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
	doc.MetaName = vgName

	clauses := xslices.Map(pvDisks, func(d string) string {
		return fmt.Sprintf(`disk.dev_path == "%s"`, d)
	})
	expr := strings.Join(clauses, " || ")

	doc.PhysicalVolumes.VolumeSelector.Match = cel.MustExpression(
		cel.ParseBooleanExpression(expr, celenv.DiskLocator()),
	)

	return doc
}

// provisionVGViaConfig applies the doc, waits for PV + VG status.
//
//nolint:gocyclo
func (suite *StorageSuite) provisionVGViaConfig(nodeCtx context.Context, node, nodeName string, pvDisks []string) {
	suite.T().Logf("provisioning VG %q on %s/%s with %v", vgName, node, nodeName, pvDisks)

	suite.PatchMachineConfig(nodeCtx, newVGConfig(pvDisks))

	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	expectedPVIDs := xslices.ToSet(xslices.Map(pvDisks, func(d string) string {
		return strings.TrimPrefix(strings.ReplaceAll(d, "/", "-"), "-dev-")
	}))

	suite.Require().Eventually(func() bool {
		pvs, err := safe.StateListAll[*storageres.LVMPhysicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		found := map[string]struct{}{}

		for pv := range pvs.All() {
			id := pv.Metadata().ID()
			if _, ok := expectedPVIDs[id]; !ok {
				continue
			}

			spec := pv.TypedSpec()
			if spec.VGName != vgName || spec.UUID == "" {
				continue
			}

			found[id] = struct{}{}
		}

		return len(found) == len(expectedPVIDs)
	}, assertTimeout, assertInterval, "PV statuses not reported for %v", pvDisks)

	suite.Require().Eventually(func() bool {
		vg, err := safe.StateGetByID[*storageres.LVMVolumeGroupStatus](nodeCtx, suite.Client.COSI, vgName)
		if err != nil {
			return false
		}

		spec := vg.TypedSpec()

		return spec.Name == vgName &&
			spec.PVCount == fmt.Sprintf("%d", len(pvDisks)) &&
			spec.Size != "" && spec.Size != "0" &&
			spec.UUID != ""
	}, assertTimeout, assertInterval, "VG status not reported")
}

// TestLVMStatus tests declarative VG provisioning + LV status surfacing.
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
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	pvDisks := userDisks[:2]

	nodeCtx := client.WithNode(suite.ctx, node)

	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	// LV creation not declarative yet; use lvcreate via privileged pod.
	podDef, err := suite.NewPrivilegedPod("lvm-status")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	stdout, _, err := podDef.Exec(
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

	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	expectedLVPaths := xslices.ToSet([]string{"/dev/vg0/lv0", "/dev/vg0/lv1"})

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		found := map[string]struct{}{}

		for lv := range lvs.All() {
			spec := lv.TypedSpec()
			if _, ok := expectedLVPaths[spec.Path]; !ok {
				continue
			}

			if spec.VGName != vgName || spec.Size == "" || spec.Size == "0" {
				continue
			}

			found[spec.Path] = struct{}{}
		}

		return len(found) == len(expectedLVPaths)
	}, assertTimeout, assertInterval, "LV statuses not reported")

	// Drive LogicalVolumeRemove + verify status cleanup.
	for _, lvName := range []string{"lv0", "lv1"} {
		suite.Require().NoError(suite.Client.LogicalVolumeRemove(nodeCtx, &machineapi.LVMServiceLogicalVolumeRemoveRequest{
			VolumeGroup:   vgName,
			LogicalVolume: lvName,
		}))
	}

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
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

// TestLVMActivation reboots a node and verifies the LVs are reactivated.
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
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	pvDisks := userDisks[:2]

	nodeCtx := client.WithNode(suite.ctx, node)

	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	podDef, err := suite.NewPrivilegedPod("pv-create")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	stdout, _, err := podDef.Exec(
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

	suite.T().Logf("rebooting %s/%s", node, nodeName)

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
		suite.CleanupFailedPods,
	)

	suite.Require().Eventually(func() bool {
		return suite.lvmVolumeExists(node, []string{"lv0", "lv1"})
	}, 5*time.Second, 1*time.Second, "LVs were not activated after reboot")
}

// TestLVMRemove tests LV/VG/PV remove RPCs against a declarative VG.
//
//nolint:gocyclo,cyclop
func (suite *StorageSuite) TestLVMRemove() {
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
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	pvDisks := userDisks[:2]

	nodeCtx := client.WithNode(suite.ctx, node)

	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	podDef, err := suite.NewPrivilegedPod("lvm-remove")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	for _, lvName := range []string{"lv0", "lv1"} {
		stdout, _, err := podDef.Exec(
			suite.ctx,
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- lvcreate -n %s -L 64M vg0", lvName),
		)
		suite.Require().NoError(err)
		suite.Require().Contains(stdout, fmt.Sprintf("Logical volume %q created.", lvName))
	}

	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	expectedLVPaths := xslices.ToSet([]string{"/dev/vg0/lv0", "/dev/vg0/lv1"})

	// Wait for scan to observe LVs before removal.
	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		found := map[string]struct{}{}

		for lv := range lvs.All() {
			if _, ok := expectedLVPaths[lv.TypedSpec().Path]; ok {
				found[lv.TypedSpec().Path] = struct{}{}
			}
		}

		return len(found) == len(expectedLVPaths)
	}, assertTimeout, assertInterval, "LV statuses not reported")

	for _, lvName := range []string{"lv0", "lv1"} {
		suite.Require().NoError(suite.Client.LogicalVolumeRemove(nodeCtx, &machineapi.LVMServiceLogicalVolumeRemoveRequest{
			VolumeGroup:   vgName,
			LogicalVolume: lvName,
		}))
	}

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for lv := range lvs.All() {
			if _, ok := expectedLVPaths[lv.TypedSpec().Path]; ok {
				return false
			}
		}

		return true
	}, assertTimeout, assertInterval, "LV statuses were not cleaned up after LogicalVolumeRemove")

	// Stop the reconciler from re-creating the VG before destructive RPCs.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMVolumeGroupConfigKind, vgName)

	suite.Require().NoError(suite.Client.VolumeGroupRemove(nodeCtx, &machineapi.LVMServiceVolumeGroupRemoveRequest{
		VolumeGroup: vgName,
	}))

	suite.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*storageres.LVMVolumeGroupStatus](nodeCtx, suite.Client.COSI, vgName)

		return state.IsNotFoundError(err)
	}, assertTimeout, assertInterval, "VG status was not cleaned up after VolumeGroupRemove")

	for _, dev := range pvDisks {
		suite.Require().NoError(suite.Client.PhysicalVolumeRemove(nodeCtx, &machineapi.LVMServicePhysicalVolumeRemoveRequest{
			Device: dev,
		}))
	}

	expectedPVIDs := xslices.ToSet(xslices.Map(pvDisks, func(d string) string {
		return strings.TrimPrefix(strings.ReplaceAll(d, "/", "-"), "-dev-")
	}))

	suite.Require().Eventually(func() bool {
		pvs, err := safe.StateListAll[*storageres.LVMPhysicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for pv := range pvs.All() {
			if _, ok := expectedPVIDs[pv.Metadata().ID()]; ok {
				return false
			}
		}

		return true
	}, assertTimeout, assertInterval, "PV statuses were not cleaned up after PhysicalVolumeRemove")
}

// vgDocSelector builds an LVMVolumeGroupConfig doc with an arbitrary
// VolumeLocator CEL selector.
func vgDocSelector(name, match string) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
	doc := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
	doc.MetaName = name
	doc.PhysicalVolumes.VolumeSelector.Match = cel.MustExpression(
		cel.ParseBooleanExpression(match, celenv.VolumeLocator()),
	)

	return doc
}

// rawVolumeDoc builds a RawVolumeConfig that carves a partition out of the disk
// matched by diskMatch.
func rawVolumeDoc(name, diskMatch, maxSize string) *blockcfg.RawVolumeConfigV1Alpha1 {
	doc := blockcfg.NewRawVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(diskMatch, celenv.DiskLocator()),
	)
	doc.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
	doc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize(maxSize)

	return doc
}

// pvDeviceID derives the LVMPhysicalVolumeStatus resource ID from a device
// path (mirrors the controller's pvID helper).
func pvDeviceID(device string) string {
	return strings.TrimPrefix(strings.ReplaceAll(device, "/", "-"), "-dev-")
}

// assertPVAndVGStatus waits until every device is reported as a PV in vgName
// and the VG status surfaces with the expected PV count.
func (suite *StorageSuite) assertPVAndVGStatus(nodeCtx context.Context, vgName string, devices []string) {
	suite.assertPVStatuses(nodeCtx, vgName, devices)
	suite.assertVGStatus(nodeCtx, vgName, devices)
}

// assertPVStatuses waits until every device has a matching LVMPhysicalVolumeStatus in vgName.
func (suite *StorageSuite) assertPVStatuses(nodeCtx context.Context, vgName string, devices []string) {
	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	expectedPVIDs := xslices.ToSet(xslices.Map(devices, pvDeviceID))

	suite.Require().Eventually(func() bool {
		pvs, err := safe.StateListAll[*storageres.LVMPhysicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		found := map[string]struct{}{}

		for pv := range pvs.All() {
			if _, ok := expectedPVIDs[pv.Metadata().ID()]; !ok {
				continue
			}

			spec := pv.TypedSpec()
			if spec.VGName != vgName || spec.UUID == "" {
				continue
			}

			found[pv.Metadata().ID()] = struct{}{}
		}

		return len(found) == len(expectedPVIDs)
	}, assertTimeout, assertInterval, "PV statuses not reported for %v", devices)
}

// assertVGStatus waits until the LVMVolumeGroupStatus for vgName is fully populated.
func (suite *StorageSuite) assertVGStatus(nodeCtx context.Context, vgName string, devices []string) {
	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	suite.Require().Eventually(func() bool {
		vg, err := safe.StateGetByID[*storageres.LVMVolumeGroupStatus](nodeCtx, suite.Client.COSI, vgName)
		if err != nil {
			return false
		}

		spec := vg.TypedSpec()

		return spec.Name == vgName &&
			spec.PVCount == fmt.Sprintf("%d", len(devices)) &&
			spec.Size != "" && spec.Size != "0" &&
			spec.UUID != ""
	}, assertTimeout, assertInterval, "VG status not reported for %q", vgName)
}

// teardownVG drops the VG config and wipes the VG + PV labels off the devices.
func (suite *StorageSuite) teardownVG(nodeCtx context.Context, vgName string, devices []string) {
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMVolumeGroupConfigKind, vgName)

	if err := suite.Client.VolumeGroupRemove(nodeCtx, &machineapi.LVMServiceVolumeGroupRemoveRequest{
		VolumeGroup: vgName,
	}); !isAlreadyGone(err) {
		suite.T().Logf("VolumeGroupRemove %q failed: %v", vgName, err)
	}

	for _, dev := range devices {
		if err := suite.Client.PhysicalVolumeRemove(nodeCtx, &machineapi.LVMServicePhysicalVolumeRemoveRequest{
			Device: dev,
		}); !isAlreadyGone(err) {
			suite.T().Logf("PhysicalVolumeRemove %s failed: %v", dev, err)
		}
	}
}

// cleanupRawVG fully reclaims a raw-volume-backed VG: stop the reconciler,
// remove the VG (cascades, frees PVs), drop the raw volume configs, then wipe
// the PV partition devices and the disk. A whole-disk wipe alone is NOT enough:
// the lvm2-pv signature lives at the partition offset (retained non-
// destructively when the raw volume is removed) and would otherwise survive to
// trip up later tests reusing the same disk.
func (suite *StorageSuite) cleanupRawVG(nodeCtx context.Context, vgName string, rawNames, devices []string, disk string) {
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMVolumeGroupConfigKind, vgName)

	if err := suite.Client.VolumeGroupRemove(nodeCtx, &machineapi.LVMServiceVolumeGroupRemoveRequest{
		VolumeGroup: vgName,
	}); !isAlreadyGone(err) {
		suite.T().Logf("VolumeGroupRemove %q failed: %v", vgName, err)
	}

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.RawVolumeConfigKind, rawNames...)

	// Wipe each partition device to clear its lvm2-pv label, then drop it.
	// A partition that is already gone (e.g. dropped by a previous step) is
	// fine: the goal is a clean device, and the whole-disk wipe below clears
	// any residual partition table regardless.
	for _, dev := range devices {
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			err := suite.Client.BlockDeviceWipe(nodeCtx, &storageapi.BlockDeviceWipeRequest{
				Devices: []*storageapi.BlockDeviceWipeDescriptor{
					{
						Device:        filepath.Base(dev),
						Method:        storageapi.BlockDeviceWipeDescriptor_FAST,
						DropPartition: true,
					},
				},
			})
			if isAlreadyGone(err) {
				return
			}

			assert.NoError(collect, err)
		}, time.Minute, time.Second, "failed to wipe partition %s", dev)
	}

	// Drop any residual partition table on the disk.
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		assert.NoError(collect, suite.Client.BlockDeviceWipe(nodeCtx, &storageapi.BlockDeviceWipeRequest{
			Devices: []*storageapi.BlockDeviceWipeDescriptor{
				{
					Device: filepath.Base(disk),
					Method: storageapi.BlockDeviceWipeDescriptor_FAST,
				},
			},
		}))
	}, time.Minute, time.Second, "failed to wipe disk %s", disk)
}

// provisionRawVolumes creates raw volumes named lvmpv<i> on the disk matched by
// diskMatch and returns their partition device paths.
func (suite *StorageSuite) provisionRawVolumes(nodeCtx context.Context, diskMatch string, names ...string) []string {
	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	docs := xslices.Map(names, func(name string) any {
		return rawVolumeDoc(name, diskMatch, "1GiB")
	})

	suite.PatchMachineConfig(nodeCtx, docs...)

	devices := make([]string, 0, len(names))

	for _, name := range names {
		rawVolumeID := constants.RawVolumePrefix + name

		suite.Require().Eventually(func() bool {
			vs, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, rawVolumeID)
			if err != nil {
				return false
			}

			return vs.TypedSpec().Phase == block.VolumePhaseReady && vs.TypedSpec().Location != ""
		}, assertTimeout, assertInterval, "raw volume %q not ready", rawVolumeID)

		vs, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, rawVolumeID)
		suite.Require().NoError(err)

		devices = append(devices, vs.TypedSpec().Location)
	}

	return devices
}

// TestLVMOnRawVolumes provisions a VG backed by raw volume partitions selected
// by their partition label.
//
//nolint:gocyclo
func (suite *StorageSuite) TestLVMOnRawVolumes() {
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
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	nodeCtx := client.WithNode(suite.ctx, node)

	disk, err := safe.StateGetByID[*block.Disk](nodeCtx, suite.Client.COSI, filepath.Base(userDisks[0]))
	suite.Require().NoError(err)
	suite.Require().NotEmpty(disk.TypedSpec().Symlinks)

	diskMatch := fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0])

	rawNames := []string{"lvmpv0", "lvmpv1"}

	const vgRaw = "vgraw"

	var pvDevices []string

	defer func() { suite.cleanupRawVG(nodeCtx, vgRaw, rawNames, pvDevices, userDisks[0]) }()

	pvDevices = suite.provisionRawVolumes(nodeCtx, diskMatch, rawNames...)

	suite.T().Logf("raw volume partitions: %v", pvDevices)

	suite.PatchMachineConfig(nodeCtx, vgDocSelector(vgRaw, `volume.partition_label.startsWith("r-lvmpv")`))

	suite.assertPVAndVGStatus(nodeCtx, vgRaw, pvDevices)
}

// TestLVMOnSpecificPVs provisions a VG on specific whole disks selected by
// device path.
func (suite *StorageSuite) TestLVMOnSpecificPVs() {
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
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	pvDisks := userDisks[:2]

	nodeCtx := client.WithNode(suite.ctx, node)

	const vgSpecific = "vgspecific"

	clauses := xslices.Map(pvDisks, func(d string) string {
		return fmt.Sprintf(`disk.dev_path == "%s"`, d)
	})

	suite.PatchMachineConfig(nodeCtx, vgDocSelector(vgSpecific, strings.Join(clauses, " || ")))

	defer suite.teardownVG(nodeCtx, vgSpecific, pvDisks)

	suite.assertPVAndVGStatus(nodeCtx, vgSpecific, pvDisks)
}

// TestLVMOverlapConflict applies two VGs whose selectors overlap and verifies
// the conflict is surfaced as an LVMValidationError while only one VG claims
// the shared device.
//
//nolint:gocyclo
func (suite *StorageSuite) TestLVMOverlapConflict() {
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
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	sharedDisk := userDisks[0]

	nodeCtx := client.WithNode(suite.ctx, node)

	const (
		vgWin  = "vgwin"
		vgLose = "vglose"
	)

	match := fmt.Sprintf(`disk.dev_path == "%s"`, sharedDisk)

	// vgWin is listed first, so it wins the shared device; vgLose conflicts.
	suite.PatchMachineConfig(nodeCtx, vgDocSelector(vgWin, match), vgDocSelector(vgLose, match))

	// Cleanup: drop BOTH configs before wiping. Otherwise removing one VG lets
	// the other (still configured for the same disk) win and re-provision,
	// leaving an orphaned PV/VG behind.
	defer func() {
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMVolumeGroupConfigKind, vgWin, vgLose)

		for _, vg := range []string{vgWin, vgLose} {
			if err := suite.Client.VolumeGroupRemove(nodeCtx, &machineapi.LVMServiceVolumeGroupRemoveRequest{
				VolumeGroup: vg,
			}); !isAlreadyGone(err) {
				suite.T().Logf("VolumeGroupRemove %q failed: %v", vg, err)
			}
		}

		if err := suite.Client.PhysicalVolumeRemove(nodeCtx, &machineapi.LVMServicePhysicalVolumeRemoveRequest{
			Device: sharedDisk,
		}); !isAlreadyGone(err) {
			suite.T().Logf("PhysicalVolumeRemove %s failed: %v", sharedDisk, err)
		}
	}()

	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	// The losing VG surfaces a validation error referencing the winner.
	suite.Require().Eventually(func() bool {
		e, err := safe.StateGetByID[*storageres.LVMValidationError](nodeCtx, suite.Client.COSI, vgLose)
		if err != nil {
			return false
		}

		return e.TypedSpec().VGName == vgLose && strings.Contains(e.TypedSpec().Message, vgWin)
	}, assertTimeout, assertInterval, "validation error not surfaced for %q", vgLose)

	// The winning VG actually claims the device.
	suite.assertPVAndVGStatus(nodeCtx, vgWin, []string{sharedDisk})
}

// TestLVMRawVolumeRemoveInUse provisions a VG on a raw volume partition, then
// removes the RawVolumeConfig and verifies the removal is non-destructive: the
// VolumeStatus goes away, but the partition is retained as an LVM PV and the VG
// keeps working.
//
//nolint:gocyclo
func (suite *StorageSuite) TestLVMRawVolumeRemoveInUse() {
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
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	nodeCtx := client.WithNode(suite.ctx, node)

	disk, err := safe.StateGetByID[*block.Disk](nodeCtx, suite.Client.COSI, filepath.Base(userDisks[0]))
	suite.Require().NoError(err)
	suite.Require().NotEmpty(disk.TypedSpec().Symlinks)

	diskMatch := fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0])

	const (
		rawName = "lvmpv0"
		vgRaw   = "vgrawremove"
	)

	rawNames := []string{rawName}

	var pvDevices []string

	defer func() { suite.cleanupRawVG(nodeCtx, vgRaw, rawNames, pvDevices, userDisks[0]) }()

	pvDevices = suite.provisionRawVolumes(nodeCtx, diskMatch, rawName)
	partition := pvDevices[0]

	suite.PatchMachineConfig(nodeCtx, vgDocSelector(vgRaw, `volume.partition_label.startsWith("r-lvmpv")`))

	suite.assertPVAndVGStatus(nodeCtx, vgRaw, pvDevices)

	// Remove the raw volume config: its VolumeStatus must disappear.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.RawVolumeConfigKind, rawName)

	rawVolumeID := constants.RawVolumePrefix + rawName

	suite.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, rawVolumeID)

		return state.IsNotFoundError(err)
	}, 90*time.Second, 2*time.Second, "raw volume status %q not removed", rawVolumeID)

	// Non-destructive: the partition is retained and still claimed as an LVM PV
	// (DiscoveredVolume keeps the lvm2-pv signature).
	suite.Require().Eventually(func() bool {
		dv, err := safe.StateGetByID[*block.DiscoveredVolume](nodeCtx, suite.Client.COSI, filepath.Base(partition))
		if err != nil {
			return false
		}

		return dv.TypedSpec().Name == "lvm2-pv"
	}, 90*time.Second, 2*time.Second, "partition %s was not retained as an LVM PV", partition)

	// The VG/PV status must still be present.
	suite.assertPVAndVGStatus(nodeCtx, vgRaw, pvDevices)
}

// isAlreadyGone reports whether the gRPC error means the LVM object has
// already been removed.
func isAlreadyGone(err error) bool {
	if err == nil {
		return true
	}

	return grpcstatus.Code(err) == codes.NotFound
}

// deleteLVMVolumes drops the config doc and wipes the VG + PV labels via the
// LVMService RPCs. NotFound is treated as success because the test under
// teardown may have already removed them.
func (suite *StorageSuite) deleteLVMVolumes(node string, pvDisks []string) {
	nodeCtx := client.WithNode(suite.ctx, node)

	// Drop the declarative spec first so the reconciler doesn't race the
	// wipe RPCs by re-running pvcreate / vgcreate.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMVolumeGroupConfigKind, vgName)

	if err := suite.Client.VolumeGroupRemove(nodeCtx, &machineapi.LVMServiceVolumeGroupRemoveRequest{
		VolumeGroup: vgName,
	}); !isAlreadyGone(err) {
		suite.T().Logf("VolumeGroupRemove %s failed: %v", vgName, err)
	}

	for _, dev := range pvDisks {
		if err := suite.Client.PhysicalVolumeRemove(nodeCtx, &machineapi.LVMServicePhysicalVolumeRemoveRequest{
			Device: dev,
		}); !isAlreadyGone(err) {
			suite.T().Logf("PhysicalVolumeRemove %s failed: %v", dev, err)
		}
	}
}

// lvmVolumeExists returns true once every expected LV name is visible as a
// /dev/dm-* disk symlink.
func (suite *StorageSuite) lvmVolumeExists(node string, expectedVolumes []string) bool {
	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	foundVolumes := xslices.ToSet(expectedVolumes)

	for disk := range disks.All() {
		if strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/dm") {
			for _, volumeName := range expectedVolumes {
				for _, symlink := range disk.TypedSpec().Symlinks {
					if strings.Contains(symlink, volumeName) {
						foundVolumes[volumeName] = struct{}{}
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
