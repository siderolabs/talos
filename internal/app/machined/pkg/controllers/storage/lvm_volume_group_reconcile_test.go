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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const (
	testVGName         = "vg-pool"
	testParentVolumeID = "r-lvm"
)

// fakeProvisioner records unique LVM mutations. The reconcile loop re-runs on
// every state-change event until on-disk state catches up; deduplicating by
// resource identity keeps tests assertion-friendly without forcing the fake
// to mirror the LVM scanner's behavior.
type fakeProvisioner struct {
	mu sync.Mutex

	pvCreates   map[string]struct{}
	vgCreates   map[string][]string
	vgExtends   map[string]map[string]struct{}
	activations map[string]int

	deactivations map[string]int

	pvCreateErr   error
	vgCreateErr   error
	vgExtendErr   error
	deactivateErr error
	pvCreateCalls int
}

func newFakeProvisioner() *fakeProvisioner {
	return &fakeProvisioner{
		pvCreates: map[string]struct{}{},
		vgCreates: map[string][]string{},
		vgExtends: map[string]map[string]struct{}{},

		activations:   map[string]int{},
		deactivations: map[string]int{},
	}
}

func (f *fakeProvisioner) VGChangeActivate(_ context.Context, vg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.activations[vg]++

	return nil
}

func (f *fakeProvisioner) VGChangeDeactivate(_ context.Context, vg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deactivations[vg]++

	return f.deactivateErr
}

func (f *fakeProvisioner) activationCount(vg string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.activations[vg]
}

func (f *fakeProvisioner) deactivationCount(vg string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.deactivations[vg]
}

func (f *fakeProvisioner) setDeactivateError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deactivateErr = err
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
	f.activations = map[string]int{}
	f.deactivations = map[string]int{}
	f.deactivateErr = nil
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
	suite.createVGStatusWithMissingPVs(name, "0")
}

func (suite *LVMVolumeGroupReconcileSuite) createVGStatusWithMissingPVs(name, missing string) {
	vg := storageres.NewLVMVolumeGroupStatus(storageres.NamespaceName, name)
	vg.TypedSpec().Name = name
	vg.TypedSpec().MissingPVCount = missing

	suite.Create(vg)
}

// createAssembledParentVG sets up a VG on top of a ready raw volume, as declared via provisioning.parents.
func (suite *LVMVolumeGroupReconcileSuite) createAssembledParentVG() {
	applyMachineConfig(&suite.DefaultSuite, newParentsVGDoc(testVGName, "lvm"))
	createVolumeStatus(&suite.DefaultSuite, testParentVolumeID, block.VolumePhaseReady, block.EncryptionProviderLUKS2, "/dev/vda5", "/dev/dm-0")

	suite.createPVStatus("dm-0", "/dev/dm-0", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/dm-0")
}

func (suite *LVMVolumeGroupReconcileSuite) assertParentFinalizer(present bool) {
	ctest.AssertResource(suite, testParentVolumeID, func(vs *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(present, vs.Metadata().Finalizers().Has("storage.LVMVolumeGroupReconcileController"))
	})
}

func (suite *LVMVolumeGroupReconcileSuite) eventually(check func() bool) {
	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		if check() {
			return nil
		}

		return retry.ExpectedErrorf("provisioner state not yet reached")
	})
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

func (suite *LVMVolumeGroupReconcileSuite) TestActivatesAssembledVG() {
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", testVGName)
	suite.createVGStatus(testVGName)
	suite.createVGSpec("/dev/nvme0n1")

	suite.eventually(func() bool { return suite.provisioner.activationCount(testVGName) > 0 })

	// Further events must not re-activate the VG.
	suite.createPVStatus("nvme1n1", "/dev/nvme1n1", "")
	time.Sleep(250 * time.Millisecond)

	suite.Assert().Equal(1, suite.provisioner.activationCount(testVGName))
}

func (suite *LVMVolumeGroupReconcileSuite) TestSkipsActivationWithMissingPVs() {
	suite.createPVStatus("nvme0n1", "/dev/nvme0n1", testVGName)
	suite.createVGStatusWithMissingPVs(testVGName, "1")
	suite.createVGSpec("/dev/nvme0n1")

	time.Sleep(250 * time.Millisecond)

	suite.Assert().Zero(suite.provisioner.activationCount(testVGName))
}

func (suite *LVMVolumeGroupReconcileSuite) TestDeactivatesOnParentTeardown() {
	suite.createAssembledParentVG()

	suite.assertParentFinalizer(true)
	suite.eventually(func() bool { return suite.provisioner.activationCount(testVGName) > 0 })

	// An LV is still open: the finalizer must be kept until the deactivation succeeds.
	suite.provisioner.setDeactivateError(lvm.ErrOpen)

	_, err := suite.State().Teardown(suite.Ctx(), block.NewVolumeStatus(block.NamespaceName, testParentVolumeID).Metadata())
	suite.Require().NoError(err)

	suite.eventually(func() bool { return suite.provisioner.deactivationCount(testVGName) > 0 })
	suite.assertParentFinalizer(true)

	suite.provisioner.setDeactivateError(nil)

	suite.assertParentFinalizer(false)
}

func (suite *LVMVolumeGroupReconcileSuite) TestWaitsForParentsToBeReady() {
	applyMachineConfig(&suite.DefaultSuite, newParentsVGDoc(testVGName, "lvm"))
	createVolumeStatus(&suite.DefaultSuite, testParentVolumeID, block.VolumePhaseProvisioned, block.EncryptionProviderLUKS2, "/dev/vda5", "")

	suite.createVGStatus(testVGName)
	suite.createVGSpec()

	suite.assertParentFinalizer(true)

	time.Sleep(250 * time.Millisecond)

	suite.Assert().Zero(suite.provisioner.activationCount(testVGName))
}

func (suite *LVMVolumeGroupReconcileSuite) TestReleasesParentWhenConfigRemoved() {
	suite.createAssembledParentVG()

	suite.assertParentFinalizer(true)

	suite.Destroy(config.NewMachineConfig(nil))

	suite.assertParentFinalizer(false)
	suite.Assert().Zero(suite.provisioner.deactivationCount(testVGName))
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
