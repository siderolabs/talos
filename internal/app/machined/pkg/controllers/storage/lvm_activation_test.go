// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	deactivated map[string]int    // vgName -> count

	pvScanErr     map[string]error // devicePath -> error to return from PVScanAutoActivation
	deactivateErr error            // error to return from every following VGChangeDeactivate call
}

func newFakeActivator() *fakeActivator {
	return &fakeActivator{
		pvScanned:   map[string]int{},
		vgForDevice: map[string]string{},
		activated:   map[string]int{},
		deactivated: map[string]int{},
		pvScanErr:   map[string]error{},
	}
}

func (f *fakeActivator) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pvScanned = map[string]int{}
	f.vgForDevice = map[string]string{}
	f.activated = map[string]int{}
	f.deactivated = map[string]int{}
	f.pvScanErr = map[string]error{}
	f.deactivateErr = nil
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

func (f *fakeActivator) VGChangeDeactivate(_ context.Context, vgName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deactivated[vgName]++

	return f.deactivateErr
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

func (f *fakeActivator) deactivatedCount(vg string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.deactivated[vg]
}

func (f *fakeActivator) setVGForDevice(device, vg string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.vgForDevice[device] = vg
}

func (f *fakeActivator) setPVScanErr(device string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pvScanErr[device] = err
}

func (f *fakeActivator) setDeactivateErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deactivateErr = err
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

func (suite *LVMActivationSuite) createPVStatus(id, device, vgName string) {
	pv := storageres.NewLVMPhysicalVolumeStatus(storageres.NamespaceName, id)
	pv.TypedSpec().Device = device
	pv.TypedSpec().VGName = vgName

	suite.Create(pv)
}

// createRawVolume creates a block.VolumeStatus standing in for the raw
// (possibly LUKS-decrypted) volume backing an LVM physical volume, the way
// block.VolumeManagerController would.
func (suite *LVMActivationSuite) createRawVolume(id, mountLocation string, phase blockres.VolumePhase) *blockres.VolumeStatus {
	vs := blockres.NewVolumeStatus(blockres.NamespaceName, id)
	vs.TypedSpec().Phase = phase
	vs.TypedSpec().MountLocation = mountLocation

	suite.Create(vs)

	return vs
}

func (suite *LVMActivationSuite) setVolumeStatusPhase(id string, phase blockres.VolumePhase) {
	res, err := suite.State().Get(suite.Ctx(), blockres.NewVolumeStatus(blockres.NamespaceName, id).Metadata())
	suite.Require().NoError(err)

	vs := res.(*blockres.VolumeStatus) //nolint:forcetypeassert
	vs.TypedSpec().Phase = phase

	suite.Update(vs)
}

func (suite *LVMActivationSuite) hasFinalizer(id, finalizer string) bool {
	res, err := suite.State().Get(suite.Ctx(), blockres.NewVolumeStatus(blockres.NamespaceName, id).Metadata())
	suite.Require().NoError(err)

	return res.Metadata().Finalizers().Has(finalizer)
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
// corresponding LVMPhysicalVolumeSpec at all (i.e. entirely unrelated to
// Talos config) still gets activated.
func (suite *LVMActivationSuite) TestActivatesForeignVG() {
	suite.createUdevd()
	suite.createMetaReady()
	suite.createDiscoveredPV("sdb1", "/dev/sdb1")
	suite.activator.setVGForDevice("/dev/sdb1", "foreign-vg")

	suite.eventually(func() bool {
		return suite.activator.activatedCount("foreign-vg") > 0
	})
}

func (suite *LVMActivationSuite) TestReconsidersDeviceOnceClaimedByLVMSpec() {
	suite.createUdevd()
	suite.createMetaReady()

	// First seen as blank - the controller should stop tracking
	suite.createDiscoveredVolume("nvme0n1", "/dev/nvme0n1", "")

	suite.eventually(func() bool {
		return suite.activator.pvScanCount("/dev/nvme0n1") == 0
	})

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

func (suite *LVMActivationSuite) TestFinalizesBackingVolumeWhileVGActive() {
	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", "/dev/dm-0", "sample-vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "sample-vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-sample-vg-pool"

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})
}

// The controller shall track PVs even if they are referenced by different symlinks/aliases.
func (suite *LVMActivationSuite) TestFinalizesBackingVolumeAcrossDeviceAlias() {
	dir := suite.T().TempDir()
	kernelNode := filepath.Join(dir, "dm-0")
	mapperAlias := filepath.Join(dir, "luks2-r-lvm")

	suite.Require().NoError(os.WriteFile(kernelNode, nil, 0o644))
	suite.Require().NoError(os.Symlink(kernelNode, mapperAlias))

	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", kernelNode, blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", mapperAlias, "vg-pool")
	suite.createDiscoveredPV("dm-0", mapperAlias)
	suite.activator.setVGForDevice(mapperAlias, "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})
}

// Only finalize after the backing volume is ready.
func (suite *LVMActivationSuite) TestDoesNotFinalizeVolumeNotYetReady() {
	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseProvisioned)
	suite.createPVStatus("dm-0", "/dev/dm-0", "vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.activator.activatedCount("vg-pool") > 0
	})

	time.Sleep(50 * time.Millisecond)
	suite.Assert().False(suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer),
		"must not finalize a backing volume that isn't Ready yet")

	suite.setVolumeStatusPhase(rawVolume.Metadata().ID(), blockres.VolumePhaseReady)

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})
}

func (suite *LVMActivationSuite) TestDeactivatesOnTeardownAndReleasesFinalizer() {
	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", "/dev/dm-0", "vg1")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg1")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg1"

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})

	_, err := suite.State().Teardown(suite.Ctx(), rawVolume.Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool {
		return suite.activator.deactivatedCount("vg1") > 0
	})

	suite.eventually(func() bool {
		return !suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})
}

// A backing volume can start tearing down before reconcileActivated's claim pass
// ever runs for it (e.g. activation and teardown landing on the same or adjacent ticks).
func (suite *LVMActivationSuite) TestDeactivatesEvenIfFinalizerNeverClaimed() {
	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)

	// Teardown before LVM controllers act on it
	_, err := suite.State().Teardown(suite.Ctx(), rawVolume.Metadata())
	suite.Require().NoError(err)

	suite.createPVStatus("dm-0", "/dev/dm-0", "vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.activator.activatedCount("vg-pool") > 0
	})

	suite.eventually(func() bool {
		return suite.activator.deactivatedCount("vg-pool") > 0
	})

	suite.Assert().False(suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer))
}

func (suite *LVMActivationSuite) TestDeactivatesMultiPV() {
	suite.createUdevd()
	suite.createMetaReady()

	raw1 := suite.createRawVolume("r-lvm-1", "/dev/dm-0", blockres.VolumePhaseReady)
	raw2 := suite.createRawVolume("r-lvm-2", "/dev/dm-1", blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", "/dev/dm-0", "vg-pool")
	suite.createPVStatus("dm-1", "/dev/dm-1", "vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.hasFinalizer(raw1.Metadata().ID(), finalizer) && suite.hasFinalizer(raw2.Metadata().ID(), finalizer)
	})

	_, err := suite.State().Teardown(suite.Ctx(), raw1.Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool {
		return suite.activator.deactivatedCount("vg-pool") > 0
	})

	suite.eventually(func() bool {
		return !suite.hasFinalizer(raw1.Metadata().ID(), finalizer) && !suite.hasFinalizer(raw2.Metadata().ID(), finalizer)
	})
}

func (suite *LVMActivationSuite) TestReleasesFinalizerWhenVGAlreadyGone() {
	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", "/dev/dm-0", "vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})

	suite.activator.setDeactivateErr(lvm.ErrNotFound)

	_, err := suite.State().Teardown(suite.Ctx(), rawVolume.Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool {
		return !suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})
}

// Finalizers should stay in place if an error is not caused by VG being gone, but some other cause.
func (suite *LVMActivationSuite) TestKeepsFinalizerWhenDeactivateExecutableMissing() {
	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", "/dev/dm-0", "vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})

	suite.activator.setDeactivateErr(exec.ErrNotFound)

	_, err := suite.State().Teardown(suite.Ctx(), rawVolume.Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool {
		return suite.activator.deactivatedCount("vg-pool") > 0
	})

	time.Sleep(50 * time.Millisecond)
	suite.Assert().True(suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer),
		"must not release the finalizer when the lvm executable itself couldn't be run")
}

func (suite *LVMActivationSuite) TestReactivatesVGAfterFullTeardown() {
	suite.createUdevd()
	suite.createMetaReady()

	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", "/dev/dm-0", "vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})

	_, err := suite.State().Teardown(suite.Ctx(), rawVolume.Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool {
		return suite.activator.deactivatedCount("vg-pool") > 0 && !suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})

	// Fully gone now (finalizers were released above) - simulate the
	// backing volume being destroyed and later re-provisioned.
	suite.Destroy(rawVolume)

	rawVolume2 := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)

	suite.eventually(func() bool {
		return suite.activator.activatedCount("vg-pool") > 1
	})

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume2.Metadata().ID(), finalizer)
	})
}

func (suite *LVMActivationSuite) TestDeactivatesDespiteUnrelatedScanError() {
	suite.createUdevd()
	suite.createMetaReady()

	// A VG that's already activated and finalized.
	rawVolume := suite.createRawVolume("r-lvm", "/dev/dm-0", blockres.VolumePhaseReady)
	suite.createPVStatus("dm-0", "/dev/dm-0", "vg-pool")
	suite.createDiscoveredPV("dm-0", "/dev/dm-0")
	suite.activator.setVGForDevice("/dev/dm-0", "vg-pool")

	finalizer := (&storagectrl.LVMActivationController{}).Name() + "-vg-pool"

	suite.eventually(func() bool {
		return suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
	})

	// An unrelated device whose pvscan fails on every attempt from here on.
	suite.activator.setPVScanErr("/dev/sdz1", errors.New("simulated pvscan failure"))
	suite.createDiscoveredPV("sdz1", "/dev/sdz1")

	// Tear down the backing volume of the already-activated VG.
	_, err := suite.State().Teardown(suite.Ctx(), rawVolume.Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool {
		return suite.activator.deactivatedCount("vg-pool") > 0
	})

	suite.eventually(func() bool {
		return !suite.hasFinalizer(rawVolume.Metadata().ID(), finalizer)
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
