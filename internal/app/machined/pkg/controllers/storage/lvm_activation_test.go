// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
	v1alpha1res "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// fakeActivator is a mock for storagectrl.LVMActivator.
type fakeActivator struct {
	mu sync.Mutex

	pvScanned   map[string]int    // devicePath -> count
	vgForDevice map[string]string // devicePath -> vgName to report as complete ("" = not complete yet)
	activated   map[string]int    // vgName -> count

	pvScanErr map[string]error // devicePath -> error to return from PVScanAutoActivation
}

func newFakeActivator() *fakeActivator {
	return &fakeActivator{
		pvScanned:   map[string]int{},
		vgForDevice: map[string]string{},
		activated:   map[string]int{},
		pvScanErr:   map[string]error{},
	}
}

func (f *fakeActivator) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pvScanned = map[string]int{}
	f.vgForDevice = map[string]string{}
	f.activated = map[string]int{}
	f.pvScanErr = map[string]error{}
}

func (f *fakeActivator) PVScanAutoActivation(_ context.Context, devicePath string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pvScanned[devicePath]++

	if err := f.pvScanErr[devicePath]; err != nil {
		return nil, err
	}

	vg := f.vgForDevice[devicePath]
	if vg == "" {
		return map[string]string{}, nil
	}

	return map[string]string{lvm.UdevKeyVGNameComplete: vg}, nil
}

func (f *fakeActivator) VGChangeActivate(_ context.Context, vgName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.activated[vgName]++

	return nil
}

func (f *fakeActivator) pvScanCount(device string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.pvScanned[device]
}

func (f *fakeActivator) activatedCount(vg string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.activated[vg]
}

func (f *fakeActivator) setVGForDevice(device, vg string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.vgForDevice[device] = vg
}

type LVMActivationSuite struct {
	ctest.DefaultSuite

	activator *fakeActivator
}

func (suite *LVMActivationSuite) SetupTest() {
	suite.activator.reset()
	suite.DefaultSuite.SetupTest()
}

func (suite *LVMActivationSuite) createUdevd() {
	svc := v1alpha1res.NewService("udevd")
	svc.TypedSpec().Running = true
	svc.TypedSpec().Healthy = true

	suite.Create(svc)
}

func (suite *LVMActivationSuite) createMetaReady() {
	vs := blockres.NewVolumeStatus(blockres.NamespaceName, constants.MetaPartitionLabel)
	vs.TypedSpec().Phase = blockres.VolumePhaseReady

	suite.Create(vs)
}

// ready satisfies every precondition (udevd healthy, META ready, machine
// config loaded - even with nothing declared in it) so the controller
// proceeds to actually reconcile discovered volumes.
func (suite *LVMActivationSuite) ready() {
	suite.createUdevd()
	suite.createMetaReady()
	applyMachineConfigDocs(&suite.DefaultSuite)
}

func (suite *LVMActivationSuite) createDiscoveredVolume(id, devPath, name string) {
	dv := blockres.NewDiscoveredVolume(blockres.NamespaceName, id)
	dv.TypedSpec().DevPath = devPath
	dv.TypedSpec().Name = name

	suite.Create(dv)
}

func (suite *LVMActivationSuite) createDiscoveredPV(id, devPath string) {
	suite.createDiscoveredVolume(id, devPath, "lvm2-pv")
}

func (suite *LVMActivationSuite) setDiscoveredVolumeName(id, name string) {
	res, err := suite.State().Get(suite.Ctx(), blockres.NewDiscoveredVolume(blockres.NamespaceName, id).Metadata())
	suite.Require().NoError(err)

	dv := res.(*blockres.DiscoveredVolume) //nolint:forcetypeassert
	dv.TypedSpec().Name = name

	suite.Update(dv)
}

func (suite *LVMActivationSuite) createPendingPVSpec(id, device, vgName string) {
	pv := storageres.NewLVMPhysicalVolumeSpec(storageres.NamespaceName, id)
	pv.TypedSpec().Device = device
	pv.TypedSpec().VGName = vgName

	suite.Create(pv)
}

// createVGSpec stands in for a VG declared via LVMVolumeGroupConfig, the way
// LVMVolumeGroupSpecController would produce it (one per config doc, keyed by
// VG name) - this controller only reads it, so a direct resource is enough
// to exercise the skip-if-managed behavior without pulling in the whole
// selector-resolution pipeline.
func (suite *LVMActivationSuite) createVGSpec(vgName string) *storageres.LVMVolumeGroupSpec {
	spec := storageres.NewLVMVolumeGroupSpec(storageres.NamespaceName, vgName)
	spec.TypedSpec().Name = vgName

	suite.Create(spec)

	return spec
}

func (suite *LVMActivationSuite) eventually(check func() bool) {
	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		if check() {
			return nil
		}

		return retry.ExpectedErrorf("state not yet reached")
	})
}

// TestActivatesForeignVG is the baseline: a complete VG with no
// corresponding LVMVolumeGroupSpec at all (i.e. entirely unrelated to Talos
// config) gets activated.
func (suite *LVMActivationSuite) TestActivatesForeignVG() {
	suite.ready()
	suite.createDiscoveredPV("sdb1", "/dev/sdb1")
	suite.activator.setVGForDevice("/dev/sdb1", "foreign-vg")

	suite.eventually(func() bool {
		return suite.activator.activatedCount("foreign-vg") > 0
	})
}

// TestWaitsForMachineConfig verifies the controller does not activate
// anything - including an otherwise-clearly-foreign VG - until the machine
// config has loaded, since only then can it tell whether a VG it's about to
// activate is actually declared via LVMVolumeGroupConfig.
func (suite *LVMActivationSuite) TestWaitsForMachineConfig() {
	suite.createUdevd()
	suite.createMetaReady()
	// Deliberately no applyMachineConfigDocs call.

	suite.createDiscoveredPV("sdb1", "/dev/sdb1")
	suite.activator.setVGForDevice("/dev/sdb1", "foreign-vg")

	time.Sleep(150 * time.Millisecond)
	suite.Assert().Equal(0, suite.activator.activatedCount("foreign-vg"),
		"must not activate before the machine config is loaded")

	applyMachineConfigDocs(&suite.DefaultSuite)

	suite.eventually(func() bool {
		return suite.activator.activatedCount("foreign-vg") > 0
	})
}

// TestSkipsManagedVG verifies a VG backed by a LVMVolumeGroupSpec (i.e.
// declared via LVMVolumeGroupConfig) is never activated by this controller,
// no matter how many ticks it gets - that's LVMVolumeGroupReconcileController's
// job, as an ordinary side effect of vgcreate.
func (suite *LVMActivationSuite) TestSkipsManagedVG() {
	suite.ready()
	suite.createVGSpec("vg-pool")

	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	// Give it several ticks to (wrongly) activate before asserting the negative.
	time.Sleep(150 * time.Millisecond)

	suite.Assert().Equal(0, suite.activator.activatedCount("vg-pool"),
		"must never activate a VG that has a LVMVolumeGroupSpec")
	suite.Assert().Greater(suite.activator.pvScanCount("/dev/dm-0"), 0,
		"should still be actively rechecking a managed VG's device, not writing it off")
}

// TestFallsBackToForeignOnceSpecRemoved verifies that once a VG stops being
// declared (its LVMVolumeGroupSpec goes away - e.g. the user removed the
// LVMVolumeGroupConfig), this controller picks it up as foreign rather than
// leaving it unmanaged by anyone.
func (suite *LVMActivationSuite) TestFallsBackToForeignOnceSpecRemoved() {
	suite.ready()
	spec := suite.createVGSpec("vg-pool")

	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	time.Sleep(150 * time.Millisecond)
	suite.Assert().Equal(0, suite.activator.activatedCount("vg-pool"))

	suite.Destroy(spec)

	suite.eventually(func() bool {
		return suite.activator.activatedCount("vg-pool") > 0
	})
}

func (suite *LVMActivationSuite) TestReconsidersDeviceOnceClaimedByLVMSpec() {
	suite.ready()

	// First seen as blank - the controller should stop tracking
	suite.createDiscoveredVolume("nvme0n1", "/dev/nvme0n1", "")

	// Claimed by Talos-managed Vg
	suite.createPendingPVSpec("nvme0n1", "/dev/nvme0n1", "vg-pool")

	// Should not scan yet
	time.Sleep(50 * time.Millisecond)
	suite.Assert().Equal(0, suite.activator.pvScanCount("/dev/nvme0n1"))

	// Now formatted, reprobed - should be picked up
	suite.activator.setVGForDevice("/dev/nvme0n1", "vg-pool")
	suite.setDiscoveredVolumeName("nvme0n1", "lvm2-pv")

	suite.eventually(func() bool {
		return suite.activator.activatedCount("vg-pool") > 0
	})
}

func TestLVMActivationSuite(t *testing.T) {
	t.Parallel()

	activator := newFakeActivator()

	s := &LVMActivationSuite{activator: activator}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.LVMActivationController{
				LVM: activator,
			}))
		},
	}

	suite.Run(t, s)
}
