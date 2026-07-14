// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *StorageSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

const vgName = "vg0"

// assertUserDisksReleased waits until all user disks visible before the test
// claim are visible again. It should be deferred immediately after the disk
// count check, before resource cleanup defers are registered.
func (suite *StorageSuite) assertUserDisksReleased(ctx context.Context, node, nodeName string, initial []string) {
	if suite.T().Failed() {
		return
	}

	initialCount := len(initial)

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		current := suite.UserDisks(ctx, node)
		assert.Lenf(collect, current, initialCount, "user disks were not released on %s/%s: initial %v, current %v", node, nodeName, initial, current)
	}, 30*time.Second, time.Second, "user disks were not released on %s/%s", node, nodeName)
}

// newVGConfig builds a doc whose selector matches the supplied disks.
func newVGConfig(pvDisks []string) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
	doc := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
	doc.MetaName = vgName

	clauses := xslices.Map(pvDisks, func(d string) string {
		return fmt.Sprintf(`disk.dev_path == "%s"`, d)
	})
	expr := strings.Join(clauses, " || ")

	doc.ProvisioningSpec.VolumeSelector.Match = cel.MustExpression(
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

// assertLVStatus waits until the logical volume vg/lv surfaces in
// LVMLogicalVolumeStatus with a non-zero size.
//
//nolint:unparam
func (suite *StorageSuite) assertLVStatus(nodeCtx context.Context, vg, lv string) {
	fullName := vg + "/" + lv

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for lvStatus := range lvs.All() {
			spec := lvStatus.TypedSpec()
			if spec.FullName == fullName && spec.VGName == vg && spec.Size != "" && spec.Size != "0" {
				return true
			}
		}

		return false
	}, 90*time.Second, 2*time.Second, "logical volume %q not created", fullName)
}

// lvDiskSymlink waits for the logical volume vg/lv to surface as a
// device-mapper block.Disk and returns one of its udev symlinks (e.g.
// /dev/mapper/<vg>-<lv>), usable in a UserVolume disk selector.
func (suite *StorageSuite) lvDiskSymlink(nodeCtx context.Context, vg, lv string) string {
	marker := vg + "-" + lv

	var symlink string

	suite.Require().Eventually(func() bool {
		disks, err := safe.StateListAll[*block.Disk](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for disk := range disks.All() {
			if !strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/dm") {
				continue
			}

			for _, s := range disk.TypedSpec().Symlinks {
				if strings.Contains(s, marker) {
					symlink = s

					return true
				}
			}
		}

		return false
	}, 90*time.Second, 2*time.Second, "logical volume %q/%q did not surface as a disk", vg, lv)

	return symlink
}

// TestLVMUserVolumeOnLogicalVolume provisions a VG spanning three PVs, a linear
// logical volume that occupies the whole VG (and therefore spans all three
// PVs), then a UserVolume of type=disk backed by that logical volume. All steps
// are driven purely through Talos config + resources (no privileged pods).
//
//nolint:gocyclo,cyclop
func (suite *StorageSuite) TestLVMUserVolumeOnLogicalVolume() {
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

	if len(userDisks) < 3 {
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

	pvDisks := userDisks[:3]

	nodeCtx := client.WithNode(suite.ctx, node)

	const (
		lvName  = "lvspan"
		userVol = "lvmuser"
	)

	// VG over all three disks.
	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	// Linear LV occupying the whole VG -> spans all three PVs.
	suite.PatchMachineConfig(nodeCtx, lvDoc(suite.T(), lvName, vgName, "linear", "100%"))

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMLogicalVolumeConfigKind, lvName)

	suite.assertLVStatus(nodeCtx, vgName, lvName)

	// The LV must be larger than any single PV, proving it spans more than one.
	suite.assertLVSpansDisk(nodeCtx, vgName, lvName, pvDisks[0])

	symlink := suite.lvDiskSymlink(nodeCtx, vgName, lvName)

	suite.T().Logf("provisioning UserVolume %q on logical volume %s/%s (%s)", userVol, vgName, lvName, symlink)

	uv := blockcfg.NewUserVolumeConfigV1Alpha1()
	uv.MetaName = userVol
	uv.VolumeType = new(block.VolumeTypeDisk)
	uv.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", symlink), celenv.DiskLocator()),
	)
	uv.FilesystemSpec.FilesystemType = block.FilesystemTypeXFS

	suite.PatchMachineConfig(nodeCtx, uv)

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, userVol)

	userVolumeID := constants.UserVolumePrefix + userVol

	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	suite.Require().Eventually(func() bool {
		vs, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, userVolumeID)
		if err != nil {
			return false
		}

		return vs.TypedSpec().Phase == block.VolumePhaseReady
	}, assertTimeout, assertInterval, "user volume %q not ready", userVolumeID)

	suite.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*block.MountStatus](nodeCtx, suite.Client.COSI, userVolumeID)

		return err == nil
	}, assertTimeout, assertInterval, "user volume %q not mounted", userVolumeID)
}

// assertLVSpansDisk asserts the logical volume vg/lv is larger than the single
// disk, which (for a linear 100%VG volume) demonstrates it spans multiple PVs.
func (suite *StorageSuite) assertLVSpansDisk(nodeCtx context.Context, vg, lv, disk string) {
	fullName := vg + "/" + lv

	diskRes, err := safe.StateGetByID[*block.Disk](nodeCtx, suite.Client.COSI, filepath.Base(disk))
	suite.Require().NoError(err)

	diskSize := diskRes.TypedSpec().Size

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for lvStatus := range lvs.All() {
			spec := lvStatus.TypedSpec()
			if spec.FullName != fullName {
				continue
			}

			lvSize, perr := strconv.ParseUint(spec.Size, 10, 64)
			if perr != nil {
				return false
			}

			return lvSize > diskSize
		}

		return false
	}, 90*time.Second, 2*time.Second, "logical volume %q did not span more than one PV (disk size %d)", fullName, diskSize)
}

// TestLVMActivation provisions a VG + LV declaratively, reboots the node, and
// verifies the logical volume is reactivated.
func (suite *StorageSuite) TestLVMActivation() {
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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

	pvDisks := userDisks[:2]

	nodeCtx := client.WithNode(suite.ctx, node)

	const lvName = "lvdata"

	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	suite.PatchMachineConfig(nodeCtx, lvDoc(suite.T(), lvName, vgName, "linear", "1GiB"))

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMLogicalVolumeConfigKind, lvName)

	suite.assertLVStatus(nodeCtx, vgName, lvName)

	suite.T().Logf("rebooting %s/%s", node, nodeName)

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
		suite.CleanupFailedPods,
	)

	suite.Require().Eventually(func() bool {
		return suite.lvmVolumeExists(node, []string{lvName})
	}, 60*time.Second, 2*time.Second, "LV not activated after reboot")
}

// TestLVMRaid1LogicalVolume provisions a VG over two PVs and a raid1 (mirrored)
// logical volume, then verifies the LV surfaces with a raid1 layout.
func (suite *StorageSuite) TestLVMRaid1LogicalVolume() {
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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

	pvDisks := userDisks[:2]

	nodeCtx := client.WithNode(suite.ctx, node)

	const lvName = "lvmirror"

	// raid1 mirrors across two PVs.
	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	suite.PatchMachineConfig(nodeCtx, lvDoc(suite.T(), lvName, vgName, "raid1", "1GiB"))

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMLogicalVolumeConfigKind, lvName)

	suite.assertLVStatus(nodeCtx, vgName, lvName)
	suite.assertLVLayout(nodeCtx, vgName, lvName, "raid1")
}

// assertLVLayout waits until the logical volume vg/lv reports a layout string
// containing want (e.g. "linear", "raid0", "raid1", "raid10").
func (suite *StorageSuite) assertLVLayout(nodeCtx context.Context, vg, lv, want string) {
	fullName := vg + "/" + lv

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for lvStatus := range lvs.All() {
			spec := lvStatus.TypedSpec()
			if spec.FullName == fullName && strings.Contains(spec.Layout, want) {
				return true
			}
		}

		return false
	}, 90*time.Second, 2*time.Second, "logical volume %q did not report %q layout", fullName, want)
}

// provisionLVOfType provisions a VG over the first pvCount user disks and a
// logical volume of the given type, asserting the LV surfaces with the expected
// layout. Skips when the node has too few user disks. Returns once verified;
// teardown is registered via t.Cleanup-style defers by the caller.
func (suite *StorageSuite) runLVTypeTest(lvType, wantLayout string, pvCount int) {
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

	if len(userDisks) < pvCount {
		suite.T().Skipf("not enough user disks on %s/%s for %s (need %d): %q", node, nodeName, lvType, pvCount, userDisks)
	}

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

	pvDisks := userDisks[:pvCount]

	nodeCtx := client.WithNode(suite.ctx, node)

	const lvName = "lvtyped"

	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	suite.PatchMachineConfig(nodeCtx, lvDoc(suite.T(), lvName, vgName, lvType, "1GiB"))

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMLogicalVolumeConfigKind, lvName)

	suite.assertLVStatus(nodeCtx, vgName, lvName)
	suite.assertLVLayout(nodeCtx, vgName, lvName, wantLayout)
}

// TestLVMLinearLogicalVolume provisions a linear LV and checks its layout.
func (suite *StorageSuite) TestLVMLinearLogicalVolume() {
	suite.runLVTypeTest("linear", "linear", 1)
}

// TestLVMRaid0LogicalVolume provisions a raid0 (striped) LV across two PVs.
func (suite *StorageSuite) TestLVMRaid0LogicalVolume() {
	suite.runLVTypeTest("raid0", "raid0", 2)
}

// TestLVMRaid10LogicalVolume provisions a raid10 (striped mirror) LV; with the
// default mirror count of 1 and auto stripes this needs at least four PVs.
func (suite *StorageSuite) TestLVMRaid10LogicalVolume() {
	suite.runLVTypeTest("raid10", "raid10", 4)
}

// lvObservedSize returns the observed size in bytes of the logical volume
// vg/lv, or 0 if it is not (yet) reported.
//
//nolint:unparam
func (suite *StorageSuite) lvObservedSize(nodeCtx context.Context, vg, lv string) uint64 {
	fullName := vg + "/" + lv

	lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	for lvStatus := range lvs.All() {
		spec := lvStatus.TypedSpec()
		if spec.FullName != fullName {
			continue
		}

		size, perr := strconv.ParseUint(spec.Size, 10, 64)
		if perr != nil {
			return 0
		}

		return size
	}

	return 0
}

// TestLVMGrowLogicalVolume verifies that raising the size of an
// LVMLogicalVolumeConfig grows the existing LV, both for an absolute (bytes)
// size and a percentage-of-VG size. Shrinking never happens, so a successful
// grow is observed as a strictly larger LV size.
//
//nolint:gocyclo
func (suite *StorageSuite) TestLVMGrowLogicalVolume() {
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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

	pvDisks := userDisks[:2]

	nodeCtx := client.WithNode(suite.ctx, node)

	const (
		lvBytes   = "lvbytes"
		lvPercent = "lvpercent"
	)

	suite.provisionVGViaConfig(nodeCtx, node, nodeName, pvDisks)

	defer suite.deleteLVMVolumes(node, pvDisks)

	// One byte-sized LV and one percent-sized LV, both small to start.
	suite.PatchMachineConfig(
		nodeCtx,
		lvDoc(suite.T(), lvBytes, vgName, "linear", "1GiB"),
		lvDoc(suite.T(), lvPercent, vgName, "linear", "10%"),
	)

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMLogicalVolumeConfigKind, lvBytes, lvPercent)

	suite.assertLVStatus(nodeCtx, vgName, lvBytes)
	suite.assertLVStatus(nodeCtx, vgName, lvPercent)

	bytesBefore := suite.lvObservedSize(nodeCtx, vgName, lvBytes)
	percentBefore := suite.lvObservedSize(nodeCtx, vgName, lvPercent)

	suite.Require().NotZero(bytesBefore)
	suite.Require().NotZero(percentBefore)

	suite.T().Logf("growing %s 1GiB->3GiB, %s 10%%->40%%", lvBytes, lvPercent)

	// Raise both sizes.
	suite.PatchMachineConfig(
		nodeCtx,
		lvDoc(suite.T(), lvBytes, vgName, "linear", "3GiB"),
		lvDoc(suite.T(), lvPercent, vgName, "linear", "40%"),
	)

	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	suite.Require().Eventually(func() bool {
		return suite.lvObservedSize(nodeCtx, vgName, lvBytes) > bytesBefore
	}, assertTimeout, assertInterval, "byte-sized LV did not grow")

	suite.Require().Eventually(func() bool {
		return suite.lvObservedSize(nodeCtx, vgName, lvPercent) > percentBefore
	}, assertTimeout, assertInterval, "percent-sized LV did not grow")
}

// vgDocSelector builds an LVMVolumeGroupConfig doc with an arbitrary
// VolumeLocator CEL selector.
func vgDocSelector(name, match string) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
	doc := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
	doc.MetaName = name
	doc.ProvisioningSpec.VolumeSelector.Match = cel.MustExpression(
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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

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

// lvDoc builds an LVMLogicalVolumeConfig in the given VG.
func lvDoc(t *testing.T, name, vg, lvType, maxSize string) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
	t.Helper()

	parsedType, err := storageres.LVMLogicalVolumeTypeString(lvType)
	require.NoError(t, err)

	doc := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.LVType = parsedType
	doc.Provisioning.VolumeGroup = vg
	doc.Provisioning.ProvisioningMaxSize = blockcfg.MustSize(maxSize)

	return doc
}

// TestLVMLogicalVolume provisions a VG and a logical volume inside it
// declaratively, then verifies the LV status surfaces and tears everything
// down via the LVMService RPCs.
//
//nolint:gocyclo
func (suite *StorageSuite) TestLVMLogicalVolume() {
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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

	pvDisks := userDisks[:1]

	nodeCtx := client.WithNode(suite.ctx, node)

	const (
		vgName = "vglv"
		lvName = "lv-data"
	)

	match := fmt.Sprintf(`disk.dev_path == "%s"`, pvDisks[0])

	suite.PatchMachineConfig(nodeCtx, vgDocSelector(vgName, match))

	defer suite.teardownVG(nodeCtx, vgName, pvDisks)

	suite.assertPVAndVGStatus(nodeCtx, vgName, pvDisks)

	// Now declare a logical volume inside the VG.
	suite.PatchMachineConfig(nodeCtx, lvDoc(suite.T(), lvName, vgName, "linear", "1GiB"))

	defer func() {
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.LVMLogicalVolumeConfigKind, lvName)

		if err := suite.Client.LogicalVolumeRemove(nodeCtx, &machineapi.LVMServiceLogicalVolumeRemoveRequest{
			VolumeGroup:   vgName,
			LogicalVolume: lvName,
		}); !isAlreadyGone(err) {
			suite.T().Logf("LogicalVolumeRemove %s/%s failed: %v", vgName, lvName, err)
		}
	}()

	fullName := vgName + "/" + lvName

	const (
		assertTimeout  = 90 * time.Second
		assertInterval = 2 * time.Second
	)

	suite.Require().Eventually(func() bool {
		lvs, err := safe.StateListAll[*storageres.LVMLogicalVolumeStatus](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for lv := range lvs.All() {
			spec := lv.TypedSpec()
			if spec.FullName == fullName && spec.VGName == vgName && spec.Size != "" && spec.Size != "0" {
				return true
			}
		}

		return false
	}, assertTimeout, assertInterval, "logical volume %q not created", fullName)
}

// TestLVMOverlapConflict applies two VGs whose selectors overlap and verifies
// the conflict is surfaced as an LVMValidationError while only one VG claims
// the shared device.
//
//nolint:gocyclo
func (suite *StorageSuite) TestLVMOverlapConflict() {
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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

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

	defer suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)

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

// raidDoc builds a RAIDArrayConfig whose selector matches the supplied
// member disks by device path.
func raidDoc(name string, level storageres.MDLevel, memberDisks []string) *storagecfg.RAIDArrayConfigV1Alpha1 {
	doc := storagecfg.NewRAIDArrayConfigV1Alpha1()
	doc.MetaName = name
	doc.Level = level

	clauses := xslices.Map(memberDisks, func(d string) string {
		return fmt.Sprintf(`disk.dev_path == "%s"`, d)
	})

	doc.ProvisioningSpec.RAIDVolumeSelector.Match = cel.MustExpression(
		cel.ParseBooleanExpression(strings.Join(clauses, " || "), celenv.DiskLocator()),
	)

	return doc
}

const (
	raidAssertTimeout  = 10 * time.Minute // array rebuild can take a while
	raidAssertInterval = 15 * time.Second
)

// provisionRAID1Mirror provisions a raid1 (mirror) MD array named raidName across
// the first two user disks declaratively via a RAIDArrayConfig, waits for the
// MDArrayStatus to surface with both members and the stable by-id device (Ready or
// still rebuilding). minDisks lets callers require extra spare disks (e.g. the
// grow test needs a third). Each test must pass a unique raidName so its array and
// member disks do not collide with the other RAID tests in this suite.
//
// It returns the node context, the stable by-id device path, the full user-disk
// list, and a teardown func the caller MUST defer. teardown wipes and destroys the
// array (freeing the member disks) and asserts the disks are released.
//
//nolint:gocyclo
func (suite *StorageSuite) provisionRAID1Mirror(raidName string, minDisks int) (nodeCtx context.Context, expectedDevice string, userDisks []string, teardown func()) {
	suite.T().Helper()

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

	userDisks = suite.UserDisks(suite.ctx, node)

	if len(userDisks) < minDisks {
		suite.T().Skipf("not enough user disks on %s/%s: %q", node, nodeName, userDisks)
	}

	memberDisks := userDisks[:2]

	nodeCtx = client.WithNode(suite.ctx, node)

	// mdadm records the array name prefixed with the "talos" homehost (see
	// md.DevicePath), so the stable by-id alias is md-name-talos:<name>.
	expectedDevice = "/dev/disk/by-id/md-name-talos:" + raidName

	// Drop the config first so the reconciler stops managing the array, then
	// destroy it (stops the array + zeroes member superblocks) so the disks are
	// reusable by later tests. NotFound means it is already gone. The disk-release
	// check runs last, after the array is torn down.
	teardown = func() {
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.RAIDArrayConfigKind, raidName)

		// MDDestroy zeroes the md superblocks but NOT the array's data area, so a
		// filesystem written on top of the array (e.g. a whole-disk UserVolume)
		// leaves a stale signature at the member disks' data offset that would
		// resurface in the next array assembled from the same disks and trip up the
		// volume manager's format check ("wrong format"). Wiping the members can't
		// reach that offset; wipe the array device itself (its signatures map
		// straight to the members' data area) BEFORE destroying it. The array may
		// still be briefly held by a user volume being torn down (its config is
		// removed by the test's own defer, which runs first), so retry.
		if devName, ok := suite.raidArrayDevName(nodeCtx, raidName); ok {
			suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
				err := suite.Client.BlockDeviceWipe(nodeCtx, &storageapi.BlockDeviceWipeRequest{
					Devices: []*storageapi.BlockDeviceWipeDescriptor{
						{
							Device: devName,
							Method: storageapi.BlockDeviceWipeDescriptor_FAST,
						},
					},
				})
				if isAlreadyGone(err) {
					return
				}

				assert.NoError(collect, err)
			}, 3*time.Minute, 5*time.Second, "failed to wipe MD array device %s", devName)
		}

		// Stop the array + zero member superblocks. NotFound means already gone.
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			err := suite.Client.MDDestroy(nodeCtx, &machineapi.MDDestroyRequest{Device: expectedDevice})
			if isAlreadyGone(err) {
				return
			}

			assert.NoError(collect, err)
		}, 3*time.Minute, 5*time.Second, "failed to destroy MD array %q", expectedDevice)

		suite.assertUserDisksReleased(suite.ctx, node, nodeName, userDisks)
	}

	suite.T().Logf("provisioning raid1 array %q on %s/%s with %v", raidName, node, nodeName, memberDisks)

	suite.PatchMachineConfig(nodeCtx, raidDoc(raidName, storageres.MDLevelRAID1, memberDisks))

	suite.Require().Eventually(func() bool {
		return suite.mdArrayStatusMatches(nodeCtx, raidName, expectedDevice, memberDisks)
	}, raidAssertTimeout, raidAssertInterval, "MD array status not reported for %q", raidName)

	return nodeCtx, expectedDevice, userDisks, teardown
}

// raidArrayDevName returns the kernel device name (e.g. "md0") of the MD array
// carrying the by-id md-name symlink, if it currently exists.
func (suite *StorageSuite) raidArrayDevName(nodeCtx context.Context, raidName string) (string, bool) {
	disks, err := safe.StateListAll[*block.Disk](nodeCtx, suite.Client.COSI)
	if err != nil {
		return "", false
	}

	for disk := range disks.All() {
		for _, s := range disk.TypedSpec().Symlinks {
			if strings.Contains(s, "md-name-talos:"+raidName) {
				return filepath.Base(disk.TypedSpec().DevPath), true
			}
		}
	}

	return "", false
}

// raidDiskSymlink waits until the MD array surfaces as a block.Disk carrying the
// by-id md-name symlink and returns that symlink.
func (suite *StorageSuite) raidDiskSymlink(nodeCtx context.Context, raidName string) string {
	var raidSymlink string

	suite.Require().Eventually(func() bool {
		disks, err := safe.StateListAll[*block.Disk](nodeCtx, suite.Client.COSI)
		if err != nil {
			return false
		}

		for disk := range disks.All() {
			for _, s := range disk.TypedSpec().Symlinks {
				if strings.Contains(s, "md-name-talos:"+raidName) {
					raidSymlink = s

					return true
				}
			}
		}

		return false
	}, raidAssertTimeout, raidAssertInterval, "MD array %q did not surface as a disk", raidName)

	return raidSymlink
}

// assertRAIDUserVolumeReadyMounted waits until the user volume becomes Ready and
// mounted.
func (suite *StorageSuite) assertRAIDUserVolumeReadyMounted(nodeCtx context.Context, userVolumeID string) {
	suite.Require().Eventually(func() bool {
		vs, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, userVolumeID)
		if err != nil {
			return false
		}

		return vs.TypedSpec().Phase == block.VolumePhaseReady
	}, raidAssertTimeout, raidAssertInterval, "raid-backed user volume %q not ready", userVolumeID)

	suite.Require().Eventually(func() bool {
		_, err := safe.StateGetByID[*block.MountStatus](nodeCtx, suite.Client.COSI, userVolumeID)

		return err == nil
	}, raidAssertTimeout, raidAssertInterval, "raid-backed user volume %q not mounted", userVolumeID)
}

// TestRAIDArrayGrow provisions a raid1 array across two disks, then grows it by
// adding a third matched disk, verifying the MDArrayStatus picks up all three
// members and starts rebuilding. It intentionally does not wait for the resync to
// finish (see mdArrayGrewToMembers). The array is torn down through the MDService
// (talosctl wipe md) RPC.
func (suite *StorageSuite) TestRAIDArrayGrow_RAID1() {
	const raidName = "mdgrow"

	nodeCtx, expectedDevice, userDisks, teardown := suite.provisionRAID1Mirror(raidName, 3)
	defer teardown()

	grownMemberDisks := userDisks[:3]

	suite.T().Logf("growing raid1 array %q with %v", raidName, grownMemberDisks)

	suite.PatchMachineConfig(nodeCtx, raidDoc(raidName, storageres.MDLevelRAID1, grownMemberDisks))

	suite.Require().Eventually(func() bool {
		return suite.mdArrayStatusMatches(nodeCtx, raidName, expectedDevice, grownMemberDisks)
	}, raidAssertTimeout, raidAssertInterval, "MD array status was not grown for %q", raidName)
}

// TestRAIDArrayUserVolumeDisk provisions a disk-backed (whole-device) UserVolume
// on top of a raid1 array and verifies it comes up Ready and mounted.
func (suite *StorageSuite) TestRAIDArrayUserVolumeDisk_RAID1() {
	const raidName = "mdudisk"

	nodeCtx, _, _, teardown := suite.provisionRAID1Mirror(raidName, 2)
	defer teardown()

	raidSymlink := suite.raidDiskSymlink(nodeCtx, raidName)

	const userVol = "raiduserdisk"

	suite.T().Logf("provisioning disk UserVolume %q on raid1 array %s (%s)", userVol, raidName, raidSymlink)

	uv := blockcfg.NewUserVolumeConfigV1Alpha1()
	uv.MetaName = userVol
	uv.VolumeType = new(block.VolumeTypeDisk)
	uv.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", raidSymlink), celenv.DiskLocator()),
	)
	uv.FilesystemSpec.FilesystemType = block.FilesystemTypeXFS

	suite.PatchMachineConfig(nodeCtx, uv)

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, userVol)

	suite.assertRAIDUserVolumeReadyMounted(nodeCtx, constants.UserVolumePrefix+userVol)
}

// TestRAIDArrayUserVolumePartition provisions a partition-backed UserVolume (a
// partition carved from the raid device) on top of a raid1 array and verifies it
// comes up Ready and mounted.
func (suite *StorageSuite) TestRAIDArrayUserVolumePartition_RAID1() {
	const raidName = "mdupart"

	nodeCtx, _, _, teardown := suite.provisionRAID1Mirror(raidName, 2)
	defer teardown()

	raidSymlink := suite.raidDiskSymlink(nodeCtx, raidName)

	const userVol = "raiduserpart"

	suite.T().Logf("provisioning partition UserVolume %q on raid1 array %s (%s)", userVol, raidName, raidSymlink)

	uv := blockcfg.NewUserVolumeConfigV1Alpha1()
	uv.MetaName = userVol
	// Default VolumeType is Partition: a partition is carved out of the raid device.
	uv.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", raidSymlink), celenv.DiskLocator()),
	)
	uv.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
	uv.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("1GiB")
	uv.FilesystemSpec.FilesystemType = block.FilesystemTypeXFS

	suite.PatchMachineConfig(nodeCtx, uv)

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, userVol)

	suite.assertRAIDUserVolumeReadyMounted(nodeCtx, constants.UserVolumePrefix+userVol)
}

// mdArrayStatusMatches reports whether the array carries exactly the expected
// members and its identity fields are populated. It accepts a Ready OR Rebuilding
// array and does NOT wait for the resync to finish: every grow recovers the
// mirror, which can take far longer than the test budget. Observing the array
// with the expected members and no error is enough to prove the config
// reconciled.
//
//nolint:gocyclo
func (suite *StorageSuite) mdArrayStatusMatches(ctx context.Context, name, device string, members []string) bool {
	status, err := safe.StateGetByID[*storageres.MDArrayStatus](ctx, suite.Client.COSI, name)
	if err != nil {
		suite.T().Logf("MD array %q: status not found yet: %v", name, err)

		return false
	}

	spec := status.TypedSpec()

	var mismatches []string

	check := func(cond bool, format string, args ...any) {
		if !cond {
			mismatches = append(mismatches, fmt.Sprintf(format, args...))
		}
	}

	check(status.Metadata().ID() == name, "id: got %q, want %q", status.Metadata().ID(), name)
	check(spec.Level == storageres.MDLevelRAID1, "level: got %q, want %q", spec.Level, storageres.MDLevelRAID1)
	check(spec.Device == device, "device: got %q, want %q", spec.Device, device)
	check(spec.Error == "", "error: got %q, want empty", spec.Error)
	check(spec.RaidDevices == len(members), "raid_devices: got %d, want %d", spec.RaidDevices, len(members))
	check(spec.UUID != "", "uuid: empty")
	check(spec.Name == "talos:"+name, "name: got %q, want %q", spec.Name, "talos:"+name)
	check(spec.Metadata != "", "metadata: empty")
	check(spec.ArrayState != "", "array_state: empty")
	check(sameStringSet(spec.Members, members), "members: got %v, want %v", spec.Members, members)
	check(
		spec.Status == storageres.MDArrayPhaseReady || spec.Status == storageres.MDArrayPhaseRebuilding,
		"status: got %q, want ready or rebuilding", spec.Status,
	)

	if len(mismatches) > 0 {
		suite.T().Logf("MD array %q not matching yet: %s", name, strings.Join(mismatches, "; "))

		return false
	}

	return true
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	seen := make(map[string]struct{}, len(a))
	for _, v := range a {
		seen[v] = struct{}{}
	}

	for _, v := range b {
		if _, ok := seen[v]; !ok {
			return false
		}
	}

	return true
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

	const (
		removeTimeout  = 60 * time.Second
		removeInterval = 2 * time.Second
	)

	// Removing config docs (user volume, LV) only patches config; the actual
	// unmount + LV teardown happens asynchronously via the reconciler. Retry
	// the wipe RPCs until the LV closes ("logical volume is open") and the PVs
	// are no longer in use, otherwise the disks are never released.
	suite.Require().Eventually(func() bool {
		err := suite.Client.VolumeGroupRemove(nodeCtx, &machineapi.LVMServiceVolumeGroupRemoveRequest{
			VolumeGroup: vgName,
		})
		if !isAlreadyGone(err) {
			suite.T().Logf("VolumeGroupRemove %s failed: %v", vgName, err)
		}

		return isAlreadyGone(err)
	}, removeTimeout, removeInterval, "VolumeGroupRemove %s never succeeded", vgName)

	for _, dev := range pvDisks {
		suite.Require().Eventually(func() bool {
			err := suite.Client.PhysicalVolumeRemove(nodeCtx, &machineapi.LVMServicePhysicalVolumeRemoveRequest{
				Device: dev,
			})
			if !isAlreadyGone(err) {
				suite.T().Logf("PhysicalVolumeRemove %s failed: %v", dev, err)
			}

			return isAlreadyGone(err)
		}, removeTimeout, removeInterval, "PhysicalVolumeRemove %s never succeeded", dev)
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
