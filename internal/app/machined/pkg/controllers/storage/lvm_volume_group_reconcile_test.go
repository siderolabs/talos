// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const testVGName = "vg-pool"

// fakeProvisioner records unique LVM mutations. The reconcile loop re-runs on
// every state-change event until on-disk state catches up; deduplicating by
// resource identity keeps tests assertion-friendly without forcing the fake
// to mirror the LVM scanner's behavior.
type fakeProvisioner struct {
	mu sync.Mutex

	pvCreates   map[string]struct{}
	vgCreates   map[string][]string
	vgExtends   map[string]map[string]struct{}
	activated   map[string]int
	deactivated map[string]int

	pvCreateErr   error
	vgCreateErr   error
	vgExtendErr   error
	pvCreateCalls int
}

func newFakeProvisioner() *fakeProvisioner {
	return &fakeProvisioner{
		pvCreates:   map[string]struct{}{},
		vgCreates:   map[string][]string{},
		vgExtends:   map[string]map[string]struct{}{},
		activated:   map[string]int{},
		deactivated: map[string]int{},
	}
}

func (f *fakeProvisioner) PVCreate(_ context.Context, device string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pvCreates[device] = struct{}{}
	f.pvCreateCalls++

	return f.pvCreateErr
}

func (f *fakeProvisioner) VGCreate(_ context.Context, vg string, pvs ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.vgCreates[vg]; !ok {
		f.vgCreates[vg] = append([]string(nil), pvs...)
	}

	return f.vgCreateErr
}

func (f *fakeProvisioner) VGExtend(_ context.Context, vg string, pvs ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	set, ok := f.vgExtends[vg]
	if !ok {
		set = map[string]struct{}{}
		f.vgExtends[vg] = set
	}

	for _, pv := range pvs {
		set[pv] = struct{}{}
	}

	return f.vgExtendErr
}

func (f *fakeProvisioner) VGChangeActivate(_ context.Context, vg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.activated[vg]++

	return nil
}

func (f *fakeProvisioner) VGChangeDeactivate(_ context.Context, vg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deactivated[vg]++

	return nil
}

func (f *fakeProvisioner) activatedCount(vg string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.activated[vg]
}

func (f *fakeProvisioner) deactivatedCount(vg string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.deactivated[vg]
}

func (f *fakeProvisioner) pvCreated() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]string, 0, len(f.pvCreates))
	for d := range f.pvCreates {
		out = append(out, d)
	}

	sort.Strings(out)

	return out
}

func (f *fakeProvisioner) vgCreated() ([]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pvs, ok := f.vgCreates[testVGName]

	return append([]string(nil), pvs...), ok
}

func (f *fakeProvisioner) vgExtended(vg string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	set := f.vgExtends[vg]
	out := make([]string, 0, len(set))

	for d := range set {
		out = append(out, d)
	}

	sort.Strings(out)

	return out
}

func (f *fakeProvisioner) setPVCreateError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pvCreateErr = err
}

func (f *fakeProvisioner) pvCreateCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.pvCreateCalls
}

type LVMVolumeGroupReconcileSuite struct {
	ctest.DefaultSuite

	provisioner *fakeProvisioner
}

func (f *fakeProvisioner) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pvCreates = map[string]struct{}{}
	f.vgCreates = map[string][]string{}
	f.vgExtends = map[string]map[string]struct{}{}
	f.activated = map[string]int{}
	f.deactivated = map[string]int{}
	f.pvCreateCalls = 0
	f.pvCreateErr = nil
	f.vgCreateErr = nil
	f.vgExtendErr = nil
}

func (suite *LVMVolumeGroupReconcileSuite) SetupTest() {
	suite.provisioner.reset()
	suite.DefaultSuite.SetupTest()
}

func (suite *LVMVolumeGroupReconcileSuite) createVGSpec(pvs ...string) {
	vg := storageres.NewLVMVolumeGroupSpec(storageres.NamespaceName, testVGName)
	vg.TypedSpec().Name = testVGName
	vg.TypedSpec().PhysicalVolumes = pvs

	suite.Create(vg)
}

func (suite *LVMVolumeGroupReconcileSuite) createPVStatus(id, device, vgName string) {
	pv := storageres.NewLVMPhysicalVolumeStatus(storageres.NamespaceName, id)
	pv.TypedSpec().Device = device
	pv.TypedSpec().VGName = vgName

	suite.Create(pv)
}

func (suite *LVMVolumeGroupReconcileSuite) createVGStatus(name string) {
	vg := storageres.NewLVMVolumeGroupStatus(storageres.NamespaceName, name)
	vg.TypedSpec().Name = name

	suite.Create(vg)
}

func (suite *LVMVolumeGroupReconcileSuite) eventually(check func() bool) {
	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		if check() {
			return nil
		}

		return retry.ExpectedErrorf("provisioner state not yet reached")
	})
}

func (suite *LVMVolumeGroupReconcileSuite) hasFinalizer(volumeStatusID, finalizer string) bool {
	res, err := suite.State().Get(suite.Ctx(), block.NewVolumeStatus(block.NamespaceName, volumeStatusID).Metadata())
	suite.Require().NoError(err)

	return res.Metadata().Finalizers().Has(finalizer)
}

func (suite *LVMVolumeGroupReconcileSuite) TestCreatesPVsAndVGFromScratch() {
	suite.createVGSpec("/dev/nvme0n1", "/dev/nvme1n1")

	suite.eventually(func() bool {
		_, vgCreated := suite.provisioner.vgCreated()

		return vgCreated && len(suite.provisioner.pvCreated()) == 2
	})

	suite.Assert().Equal([]string{"/dev/nvme0n1", "/dev/nvme1n1"}, suite.provisioner.pvCreated())

	pvs, _ := suite.provisioner.vgCreated()
	suite.Assert().Equal([]string{"/dev/nvme0n1", "/dev/nvme1n1"}, pvs)
}

func (suite *LVMVolumeGroupReconcileSuite) TestSkipsPVCreateWhenAlreadyPV() {
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", "")
	suite.createVGSpec("/dev/nvme0n1", "/dev/nvme1n1")

	suite.eventually(func() bool {
		_, ok := suite.provisioner.vgCreated()

		return ok
	})

	suite.Assert().Equal([]string{"/dev/nvme1n1"}, suite.provisioner.pvCreated())
}

func (suite *LVMVolumeGroupReconcileSuite) TestExtendsExistingVG() {
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/nvme0n1", "/dev/nvme1n1")

	suite.eventually(func() bool {
		return len(suite.provisioner.vgExtended(testVGName)) > 0
	})

	suite.Assert().Equal([]string{"/dev/nvme1n1"}, suite.provisioner.vgExtended(testVGName))
	suite.Assert().Equal([]string{"/dev/nvme1n1"}, suite.provisioner.pvCreated())

	_, vgCreated := suite.provisioner.vgCreated()
	suite.Assert().False(vgCreated, "vgcreate must not be called when VG already exists")
}

func (suite *LVMVolumeGroupReconcileSuite) TestNoOpWhenObservedMatchesDesired() {
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", testVGName)
	suite.createPVStatus("nvme1n1", "/dev/nvme1n1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/nvme0n1", "/dev/nvme1n1")

	// Give the controller a chance to react; expect zero mutations.
	time.Sleep(250 * time.Millisecond)

	suite.Assert().Empty(suite.provisioner.pvCreated())
	suite.Assert().Empty(suite.provisioner.vgExtended(testVGName))

	_, vgCreated := suite.provisioner.vgCreated()
	suite.Assert().False(vgCreated)
}

// TestActivatesWhenAlreadyAssembled is the actual reported regression: a VG
// found already fully assembled (as it would be across a reboot - PVs and
// VG both already present exactly as declared) still gets an activation
// call. Before this, "additive only" meant nothing was missing here, so
// nothing ever called an activation command at all.
func (suite *LVMVolumeGroupReconcileSuite) TestActivatesWhenAlreadyAssembled() {
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", testVGName)
	suite.createPVStatus("nvme1n1", "/dev/nvme1n1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/nvme0n1", "/dev/nvme1n1")

	suite.eventually(func() bool {
		return suite.provisioner.activatedCount(testVGName) > 0
	})

	// Still additive otherwise: no pvcreate/vgcreate/vgextend for state that
	// already matches.
	suite.Assert().Empty(suite.provisioner.pvCreated())
	suite.Assert().Empty(suite.provisioner.vgExtended(testVGName))

	_, vgCreated := suite.provisioner.vgCreated()
	suite.Assert().False(vgCreated)
}

// TestActivatesAfterCreatingVG and TestActivatesAfterExtendingVG confirm
// activation isn't limited to the no-op path - it's the last step
// regardless of which branch reconcileVG took.
func (suite *LVMVolumeGroupReconcileSuite) TestActivatesAfterCreatingVG() {
	suite.createVGSpec("/dev/nvme0n1", "/dev/nvme1n1")

	suite.eventually(func() bool {
		return suite.provisioner.activatedCount(testVGName) > 0
	})
}

func (suite *LVMVolumeGroupReconcileSuite) TestActivatesAfterExtendingVG() {
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/nvme0n1", "/dev/nvme1n1")

	suite.eventually(func() bool {
		return suite.provisioner.activatedCount(testVGName) > 0
	})
}

// TestAddsFinalizerToParentRawVolume covers a VG whose provisioning.parents
// references a RawVolume: once its backing block.VolumeStatus is ready, the
// controller must both activate the VG and finalize the RawVolume so it
// cannot close out from under an active VG.
func (suite *LVMVolumeGroupReconcileSuite) TestAddsFinalizerToParentRawVolume() {
	applyMachineConfigDocs(&suite.DefaultSuite,
		newRawVolumeDoc("data1"),
		newVGParentsDoc(testVGName, storagecfg.ProvisioningVolumeParent{ParentKind: "RawVolume", ParentName: "data1"}),
	)

	createVolumeStatus(&suite.DefaultSuite, "r-data1", block.VolumePhaseReady, block.EncryptionProviderNone, "/dev/vdb1", "/dev/vdb1")
	suite.createPVStatus("vdb1", "/dev/vdb1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/vdb1")

	finalizer := (&storagectrl.LVMVolumeGroupReconcileController{}).Name() + "-" + testVGName

	suite.eventually(func() bool {
		return suite.hasFinalizer("r-data1", finalizer)
	})

	suite.eventually(func() bool {
		return suite.provisioner.activatedCount(testVGName) > 0
	})
}

// TestNoFinalizerForSelectorBasedVG confirms a selector-matched VG (no
// provisioning.parents) never gets a finalizer placed on a backing volume,
// even when one happens to exist at the matched device path - only a
// parents-based VG owns its backing volumes exclusively enough to justify
// the finalizer.
func (suite *LVMVolumeGroupReconcileSuite) TestNoFinalizerForSelectorBasedVG() {
	applyMachineConfigDocs(&suite.DefaultSuite, newVGDoc(testVGName, `disk.transport == "nvme"`))

	createVolumeStatus(&suite.DefaultSuite, "some-volume", block.VolumePhaseReady, block.EncryptionProviderNone, "/dev/nvme0n1", "/dev/nvme0n1")
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/nvme0n1")

	suite.eventually(func() bool {
		return suite.provisioner.activatedCount(testVGName) > 0
	})

	finalizer := (&storagectrl.LVMVolumeGroupReconcileController{}).Name() + "-" + testVGName

	// Give the finalizer a chance to appear if the controller wrongly added it.
	time.Sleep(250 * time.Millisecond)
	suite.Assert().False(suite.hasFinalizer("some-volume", finalizer))
}

// TestDeactivatesAndReleasesFinalizerOnParentTeardown confirms the reverse
// direction: once the parent RawVolume starts tearing down, the VG must be
// deactivated and the finalizer released so the teardown can proceed.
func (suite *LVMVolumeGroupReconcileSuite) TestDeactivatesAndReleasesFinalizerOnParentTeardown() {
	applyMachineConfigDocs(&suite.DefaultSuite,
		newRawVolumeDoc("data1"),
		newVGParentsDoc(testVGName, storagecfg.ProvisioningVolumeParent{ParentKind: "RawVolume", ParentName: "data1"}),
	)

	createVolumeStatus(&suite.DefaultSuite, "r-data1", block.VolumePhaseReady, block.EncryptionProviderNone, "/dev/vdb1", "/dev/vdb1")
	suite.createPVStatus("vdb1", "/dev/vdb1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/vdb1")

	finalizer := (&storagectrl.LVMVolumeGroupReconcileController{}).Name() + "-" + testVGName

	suite.eventually(func() bool {
		return suite.hasFinalizer("r-data1", finalizer)
	})

	_, err := suite.State().Teardown(suite.Ctx(), block.NewVolumeStatus(block.NamespaceName, "r-data1").Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool {
		return suite.provisioner.deactivatedCount(testVGName) > 0
	})

	suite.eventually(func() bool {
		return !suite.hasFinalizer("r-data1", finalizer)
	})
}

func (suite *LVMVolumeGroupReconcileSuite) TestRetriesTransientFailureWithoutNewEvent() {
	suite.provisioner.setPVCreateError(context.DeadlineExceeded)
	suite.createVGSpec("/dev/nvme0n1")

	suite.eventually(func() bool { return suite.provisioner.pvCreateCallCount() >= 1 })
	suite.provisioner.setPVCreateError(nil)

	// No resource event is emitted after backend recovery.
	suite.eventually(func() bool {
		_, ok := suite.provisioner.vgCreated()

		return ok
	})

	suite.Assert().GreaterOrEqual(suite.provisioner.pvCreateCallCount(), 2)
}

func (suite *LVMVolumeGroupReconcileSuite) TestPersistentFailureKeepsRetrying() {
	suite.provisioner.setPVCreateError(context.DeadlineExceeded)
	defer suite.provisioner.setPVCreateError(nil)

	suite.createVGSpec("/dev/nvme0n1")

	// A persistent operational error continues to retry without event input.
	suite.eventually(func() bool { return suite.provisioner.pvCreateCallCount() >= 3 })
}

func TestLVMVolumeGroupReconcileSuite(t *testing.T) {
	t.Parallel()

	provisioner := newFakeProvisioner()

	s := &LVMVolumeGroupReconcileSuite{provisioner: provisioner}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.LVMVolumeGroupReconcileController{
				LVM: provisioner,
			}))
		},
	}

	suite.Run(t, s)
}
