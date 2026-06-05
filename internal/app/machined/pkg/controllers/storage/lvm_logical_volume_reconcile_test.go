// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// fakeLVProvisioner records LVCreate / LVExtend calls keyed by "vg/lv".
type fakeLVProvisioner struct {
	mu       sync.Mutex
	created  map[string]lvm.LVCreateOptions
	extended map[string]lvm.LVExtendOptions
	err      error
}

func newFakeLVProvisioner() *fakeLVProvisioner {
	return &fakeLVProvisioner{
		created:  map[string]lvm.LVCreateOptions{},
		extended: map[string]lvm.LVExtendOptions{},
	}
}

func (f *fakeLVProvisioner) LVCreate(_ context.Context, vg, lv string, opts lvm.LVCreateOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	f.created[vg+"/"+lv] = opts

	return nil
}

func (f *fakeLVProvisioner) LVExtend(_ context.Context, vg, lv string, opts lvm.LVExtendOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	f.extended[vg+"/"+lv] = opts

	return nil
}

//nolint:unparam
func (f *fakeLVProvisioner) extendOpts(key string) (lvm.LVExtendOptions, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	opts, ok := f.extended[key]

	return opts, ok
}

func (f *fakeLVProvisioner) get(key string) (lvm.LVCreateOptions, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	opts, ok := f.created[key]

	return opts, ok
}

func (f *fakeLVProvisioner) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.created)
}

func (f *fakeLVProvisioner) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.created = map[string]lvm.LVCreateOptions{}
	f.extended = map[string]lvm.LVExtendOptions{}
}

type LVMLogicalVolumeReconcileSuite struct {
	ctest.DefaultSuite

	provisioner *fakeLVProvisioner
}

func (suite *LVMLogicalVolumeReconcileSuite) SetupTest() {
	suite.provisioner.reset()
	suite.DefaultSuite.SetupTest()
}

//nolint:unparam
func (suite *LVMLogicalVolumeReconcileSuite) createLVSpec(vg, name string, lvType storageres.LVMLogicalVolumeType, sizeBytes uint64, pct uint32) {
	lv := storageres.NewLVMLogicalVolumeSpec(storageres.NamespaceName, vg+"-"+name)
	lv.TypedSpec().VGName = vg
	lv.TypedSpec().Name = name
	lv.TypedSpec().Type = lvType
	lv.TypedSpec().SizeBytes = sizeBytes
	lv.TypedSpec().SizePercentVG = pct

	suite.Create(lv)
}

//nolint:unparam
func (suite *LVMLogicalVolumeReconcileSuite) createVGStatus(name string) {
	suite.createVGStatusSize(name, 0)
}

func (suite *LVMLogicalVolumeReconcileSuite) createVGStatusSize(name string, sizeBytes uint64) {
	vg := storageres.NewLVMVolumeGroupStatus(storageres.NamespaceName, name)
	vg.TypedSpec().Name = name

	if sizeBytes > 0 {
		vg.TypedSpec().Size = strconv.FormatUint(sizeBytes, 10)
	}

	suite.Create(vg)
}

//nolint:unparam
func (suite *LVMLogicalVolumeReconcileSuite) createPVStatus(id, device, vg string) {
	pv := storageres.NewLVMPhysicalVolumeStatus(storageres.NamespaceName, id)
	pv.TypedSpec().Device = device
	pv.TypedSpec().VGName = vg

	suite.Create(pv)
}

func (suite *LVMLogicalVolumeReconcileSuite) createLVStatus(vg, lv string) {
	suite.createLVStatusSize(vg, lv, 0)
}

func (suite *LVMLogicalVolumeReconcileSuite) createLVStatusSize(vg, lv string, sizeBytes uint64) {
	st := storageres.NewLVMLogicalVolumeStatus(storageres.NamespaceName, vg+"-"+lv)
	st.TypedSpec().FullName = vg + "/" + lv
	st.TypedSpec().VGName = vg
	st.TypedSpec().Name = lv

	if sizeBytes > 0 {
		st.TypedSpec().Size = strconv.FormatUint(sizeBytes, 10)
	}

	suite.Create(st)
}

func (suite *LVMLogicalVolumeReconcileSuite) eventually(check func() bool) {
	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		if check() {
			return nil
		}

		return retry.ExpectedErrorf("provisioner state not yet reached")
	})
}

func (suite *LVMLogicalVolumeReconcileSuite) TestCreatesLinearWhenVGReady() {
	suite.createVGStatus("vg-pool")
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 50<<30, 0)

	suite.eventually(func() bool {
		_, ok := suite.provisioner.get("vg-pool/lv-data")

		return ok
	})

	opts, _ := suite.provisioner.get("vg-pool/lv-data")
	suite.Assert().Equal("linear", opts.Type)
	suite.Assert().Equal(uint64(50<<30), opts.SizeBytes)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestWaitsForVG() {
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 1<<30, 0)

	time.Sleep(250 * time.Millisecond)

	suite.Assert().Zero(suite.provisioner.count())
}

func (suite *LVMLogicalVolumeReconcileSuite) TestSkipsExistingLV() {
	suite.createVGStatus("vg-pool")
	suite.createLVStatus("vg-pool", "lv-data")
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 1<<30, 0)

	time.Sleep(250 * time.Millisecond)

	suite.Assert().Zero(suite.provisioner.count())
}

func (suite *LVMLogicalVolumeReconcileSuite) TestRaid1RequiresTwoPVs() {
	suite.createVGStatus("vg-pool")
	suite.createPVStatus("sda1", "/dev/sda1", "vg-pool")
	suite.createLVSpec("vg-pool", "lv-mirror", storageres.LVMLogicalVolumeTypeRAID1, 1<<30, 0)

	// Only one PV -> raid1 must be skipped.
	time.Sleep(250 * time.Millisecond)
	suite.Assert().Zero(suite.provisioner.count())

	// Add a second PV -> raid1 should now be created.
	suite.createPVStatus("sdb1", "/dev/sdb1", "vg-pool")

	suite.eventually(func() bool {
		_, ok := suite.provisioner.get("vg-pool/lv-mirror")

		return ok
	})
}

//nolint:unparam
func (suite *LVMLogicalVolumeReconcileSuite) createRAIDLVSpec(vg, name string, lvType storageres.LVMLogicalVolumeType, mirrors, stripes uint32) {
	lv := storageres.NewLVMLogicalVolumeSpec(storageres.NamespaceName, vg+"-"+name)
	lv.TypedSpec().VGName = vg
	lv.TypedSpec().Name = name
	lv.TypedSpec().Type = lvType
	lv.TypedSpec().SizeBytes = 1 << 30
	lv.TypedSpec().Mirrors = mirrors
	lv.TypedSpec().Stripes = stripes

	suite.Create(lv)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestRaid0AutoStripesUsesAllPVs() {
	suite.createVGStatus("vg-pool")
	suite.createPVStatus("sda1", "/dev/sda1", "vg-pool")
	suite.createPVStatus("sdb1", "/dev/sdb1", "vg-pool")
	suite.createPVStatus("sdc1", "/dev/sdc1", "vg-pool")
	suite.createRAIDLVSpec("vg-pool", "lv-stripe", storageres.LVMLogicalVolumeTypeRAID0, 0, 0)

	suite.eventually(func() bool {
		_, ok := suite.provisioner.get("vg-pool/lv-stripe")

		return ok
	})

	opts, _ := suite.provisioner.get("vg-pool/lv-stripe")
	suite.Assert().Equal(uint32(3), opts.Stripes)
	suite.Assert().Zero(opts.Mirrors)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestRaid0RespectsExplicitStripes() {
	suite.createVGStatus("vg-pool")
	suite.createPVStatus("sda1", "/dev/sda1", "vg-pool")
	suite.createPVStatus("sdb1", "/dev/sdb1", "vg-pool")
	suite.createPVStatus("sdc1", "/dev/sdc1", "vg-pool")
	suite.createRAIDLVSpec("vg-pool", "lv-stripe", storageres.LVMLogicalVolumeTypeRAID0, 0, 2)

	suite.eventually(func() bool {
		_, ok := suite.provisioner.get("vg-pool/lv-stripe")

		return ok
	})

	opts, _ := suite.provisioner.get("vg-pool/lv-stripe")
	suite.Assert().Equal(uint32(2), opts.Stripes)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestRaid10Defaults() {
	suite.createVGStatus("vg-pool")

	for _, d := range []string{"sda1", "sdb1", "sdc1", "sdd1"} {
		suite.createPVStatus(d, "/dev/"+d, "vg-pool")
	}

	suite.createRAIDLVSpec("vg-pool", "lv-rten", storageres.LVMLogicalVolumeTypeRAID10, 0, 0)

	suite.eventually(func() bool {
		_, ok := suite.provisioner.get("vg-pool/lv-rten")

		return ok
	})

	opts, _ := suite.provisioner.get("vg-pool/lv-rten")
	suite.Assert().Equal(uint32(1), opts.Mirrors) // default mirrors
	suite.Assert().Equal(uint32(2), opts.Stripes) // 4 PVs / (1+1)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestRaid0SkipsWhenTooFewPVs() {
	suite.createVGStatus("vg-pool")
	suite.createPVStatus("sda1", "/dev/sda1", "vg-pool")
	suite.createRAIDLVSpec("vg-pool", "lv-stripe", storageres.LVMLogicalVolumeTypeRAID0, 0, 2)

	time.Sleep(250 * time.Millisecond)

	suite.Assert().Zero(suite.provisioner.count())
}

func (suite *LVMLogicalVolumeReconcileSuite) TestGrowsByBytes() {
	suite.createVGStatus("vg-pool")
	suite.createLVStatusSize("vg-pool", "lv-data", 1<<30) // observed 1GiB
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 4<<30, 0)

	suite.eventually(func() bool {
		_, ok := suite.provisioner.extendOpts("vg-pool/lv-data")

		return ok
	})

	opts, _ := suite.provisioner.extendOpts("vg-pool/lv-data")
	suite.Assert().Equal(uint64(4<<30), opts.SizeBytes)
	suite.Assert().Zero(opts.SizePercentVG)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestGrowsByPercent() {
	suite.createVGStatusSize("vg-pool", 10<<30)           // VG 10GiB
	suite.createLVStatusSize("vg-pool", "lv-data", 1<<30) // observed 1GiB, target 80% = 8GiB
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 0, 80)

	suite.eventually(func() bool {
		_, ok := suite.provisioner.extendOpts("vg-pool/lv-data")

		return ok
	})

	opts, _ := suite.provisioner.extendOpts("vg-pool/lv-data")
	suite.Assert().Equal(uint32(80), opts.SizePercentVG)
	suite.Assert().Zero(opts.SizeBytes)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestNeverShrinksBytes() {
	suite.createVGStatus("vg-pool")
	suite.createLVStatusSize("vg-pool", "lv-data", 4<<30) // observed 4GiB
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 1<<30, 0)

	// No lvextend, and a validation error is surfaced.
	ctest.AssertResource(suite, "vg-pool/lv-data", func(e *storageres.LVMValidationError, asrt *assert.Assertions) {
		asrt.Equal("vg-pool", e.TypedSpec().VGName)
		asrt.Contains(e.TypedSpec().Message, "shrinking logical volumes is not supported")
	})

	_, ok := suite.provisioner.extendOpts("vg-pool/lv-data")
	suite.Assert().False(ok)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestNeverShrinksPercent() {
	// VG shrank in this snapshot relative to the LV (or pct lowered): target
	// 20% of 10GiB = 2GiB < observed 8GiB -> shrink requested.
	suite.createVGStatusSize("vg-pool", 10<<30)
	suite.createLVStatusSize("vg-pool", "lv-data", 8<<30)
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 0, 20)

	ctest.AssertResource(suite, "vg-pool/lv-data", func(e *storageres.LVMValidationError, asrt *assert.Assertions) {
		asrt.Contains(e.TypedSpec().Message, "shrinking logical volumes is not supported")
	})

	_, ok := suite.provisioner.extendOpts("vg-pool/lv-data")
	suite.Assert().False(ok)
}

func (suite *LVMLogicalVolumeReconcileSuite) TestShrinkErrorClearedOnGrow() {
	suite.createVGStatus("vg-pool")
	suite.createLVStatusSize("vg-pool", "lv-data", 4<<30)

	lvSpec := storageres.NewLVMLogicalVolumeSpec(storageres.NamespaceName, "vg-pool-lv-data")
	lvSpec.TypedSpec().VGName = "vg-pool"
	lvSpec.TypedSpec().Name = "lv-data"
	lvSpec.TypedSpec().Type = storageres.LVMLogicalVolumeTypeLinear
	lvSpec.TypedSpec().SizeBytes = 1 << 30

	suite.Create(lvSpec)

	ctest.AssertResource(suite, "vg-pool/lv-data", func(*storageres.LVMValidationError, *assert.Assertions) {})

	// Raise the desired size above the observed -> error must clear.
	ctest.UpdateWithConflicts(suite, lvSpec, func(s *storageres.LVMLogicalVolumeSpec) error {
		s.TypedSpec().SizeBytes = 8 << 30

		return nil
	})

	ctest.AssertNoResource[*storageres.LVMValidationError](suite, "vg-pool/lv-data")
}

func (suite *LVMLogicalVolumeReconcileSuite) TestPercentNoGrowWhenAtTarget() {
	suite.createVGStatusSize("vg-pool", 10<<30)
	// observed already 80% of VG -> within slack, no grow.
	suite.createLVStatusSize("vg-pool", "lv-data", 8<<30)
	suite.createLVSpec("vg-pool", "lv-data", storageres.LVMLogicalVolumeTypeLinear, 0, 80)

	time.Sleep(250 * time.Millisecond)

	_, ok := suite.provisioner.extendOpts("vg-pool/lv-data")
	suite.Assert().False(ok)
}

func TestLVMLogicalVolumeReconcileSuite(t *testing.T) {
	t.Parallel()

	provisioner := newFakeLVProvisioner()

	s := &LVMLogicalVolumeReconcileSuite{provisioner: provisioner}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.LVMLogicalVolumeReconcileController{
				LVM: provisioner,
			}))
		},
	}

	suite.Run(t, s)
}
