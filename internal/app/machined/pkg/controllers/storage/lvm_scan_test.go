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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// fakeScanner returns canned vgs/pvs/lvs results for LVMScanController tests.
type fakeScanner struct {
	mu sync.Mutex

	vgs []lvm.VG
	pvs []lvm.PV
	lvs []lvm.LV
}

func (f *fakeScanner) VGS(context.Context) ([]lvm.VG, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]lvm.VG(nil), f.vgs...), nil
}

func (f *fakeScanner) PVS(context.Context) ([]lvm.PV, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]lvm.PV(nil), f.pvs...), nil
}

func (f *fakeScanner) LVS(context.Context) ([]lvm.LV, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]lvm.LV(nil), f.lvs...), nil
}

func (f *fakeScanner) set(vgs []lvm.VG, pvs []lvm.PV, lvs []lvm.LV) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.vgs, f.pvs, f.lvs = vgs, pvs, lvs
}

type LVMScanSuite struct {
	ctest.DefaultSuite

	scanner *fakeScanner
}

func (suite *LVMScanSuite) SetupTest() {
	suite.scanner.set(nil, nil, nil)
	suite.DefaultSuite.SetupTest()
}

func (suite *LVMScanSuite) bumpRefresh(req int) {
	r := storageres.NewLVMRefreshRequest(storageres.NamespaceName, storageres.RefreshID)
	r.TypedSpec().Request = req

	if existing, err := ctest.Get[*storageres.LVMRefreshRequest](suite, r.Metadata()); err == nil {
		ctest.UpdateWithConflicts(suite, existing, func(rr *storageres.LVMRefreshRequest) error {
			rr.TypedSpec().Request = req

			return nil
		})

		return
	}

	suite.Create(r)
}

func (suite *LVMScanSuite) TestEmitsStatusesFromScanResults() {
	const (
		vgUUID = "00000000-1111-2222-3333-df43c2bf3a03"
		pvUUID = "00000000-1111-2222-3333-6d201dd0c475"
		lvUUID = "00000000-1111-2222-3333-61a88916eddf"
	)

	suite.scanner.set(
		[]lvm.VG{{Name: "vg0", UUID: vgUUID, Size: "1000"}},
		[]lvm.PV{{Device: "/dev/sda1", VGName: "vg0", UUID: pvUUID, Size: "500"}},
		[]lvm.LV{{Path: "/dev/vg0/data", FullName: "vg0/data", Name: "data", VGName: "vg0", UUID: lvUUID, Size: "200"}},
	)

	suite.bumpRefresh(1)

	ctest.AssertResource(suite, "vg0", func(vg *storageres.LVMVolumeGroupStatus, asrt *assert.Assertions) {
		asrt.Equal(vgUUID, vg.TypedSpec().UUID)
	})

	ctest.AssertResource(suite, "sda1", func(pv *storageres.LVMPhysicalVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal("/dev/sda1", pv.TypedSpec().Device)
		asrt.Equal("vg0", pv.TypedSpec().VGName)
		asrt.Equal(pvUUID, pv.TypedSpec().UUID)
	})

	ctest.AssertResource(suite, "vg0-data", func(lv *storageres.LVMLogicalVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal("vg0/data", lv.TypedSpec().FullName)
		asrt.Equal("vg0", lv.TypedSpec().VGName)
		asrt.Equal(lvUUID, lv.TypedSpec().UUID)
	})
}

func (suite *LVMScanSuite) TestFiltersOrphanPVsWithEmptyUUID() {
	// `pvs -a` reports every block device; non-PV rows come back with an
	// empty UUID and must not produce resources.
	const orphanPVUUID = "00000000-1111-2222-3333-dbad2ea86b86"

	suite.scanner.set(
		nil,
		[]lvm.PV{
			{Device: "/dev/sda", VGName: "", UUID: ""},
			{Device: "/dev/sdb1", VGName: "", UUID: orphanPVUUID},
		},
		nil,
	)

	suite.bumpRefresh(1)

	ctest.AssertResource(suite, "sdb1", func(pv *storageres.LVMPhysicalVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(orphanPVUUID, pv.TypedSpec().UUID)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeStatus](suite, "sda")
}

func (suite *LVMScanSuite) TestEchoesRefreshCounter() {
	suite.bumpRefresh(7)

	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		status, err := ctest.Get[*storageres.LVMRefreshStatus](suite,
			storageres.NewLVMRefreshStatus(storageres.NamespaceName, storageres.RefreshID).Metadata())
		if err != nil {
			return retry.ExpectedError(err)
		}

		if status.TypedSpec().Request != 7 {
			return retry.ExpectedErrorf("counter not yet echoed; got %d", status.TypedSpec().Request)
		}

		return nil
	})
}

func (suite *LVMScanSuite) TestStaleResourcesAreCleanedUp() {
	suite.scanner.set(
		[]lvm.VG{{Name: "vg0", UUID: "00000000-1111-2222-3333-a9a231487d2c"}},
		nil,
		nil,
	)

	suite.bumpRefresh(1)

	ctest.AssertResource(suite, "vg0", func(*storageres.LVMVolumeGroupStatus, *assert.Assertions) {})

	// VG disappears on next scan; status resource should be cleaned up.
	suite.scanner.set(nil, nil, nil)

	suite.bumpRefresh(2)

	ctest.AssertNoResource[*storageres.LVMVolumeGroupStatus](suite, "vg0")
}

func TestLVMScanSuite(t *testing.T) {
	t.Parallel()

	scanner := &fakeScanner{}

	s := &LVMScanSuite{scanner: scanner}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.LVMScanController{
				LVM: scanner,
			}))
		},
	}

	suite.Run(t, s)
}
