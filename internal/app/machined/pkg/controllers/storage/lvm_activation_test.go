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
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// fakeActivator reports every PV as completing the VG named in vgByDevice.
type fakeActivator struct {
	mu sync.Mutex

	vgByDevice map[string]string
	activated  map[string]int
}

func (f *fakeActivator) PVScanAutoActivation(_ context.Context, devicePath string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return map[string]string{lvm.UdevKeyVGNameComplete: f.vgByDevice[devicePath]}, nil
}

func (f *fakeActivator) VGChangeActivate(_ context.Context, vgName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.activated[vgName]++

	return nil
}

func (f *fakeActivator) activationCount(vgName string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.activated[vgName]
}

type LVMActivationSuite struct {
	ctest.DefaultSuite

	activator *fakeActivator
}

func (suite *LVMActivationSuite) createLVMPV(id, devPath string) {
	dv := block.NewDiscoveredVolume(block.NamespaceName, id)
	dv.TypedSpec().DevPath = devPath
	dv.TypedSpec().Name = "lvm2-pv"

	suite.Create(dv)
}

func (suite *LVMActivationSuite) TestActivatesOnlyForeignVGs() {
	udevd := v1alpha1.NewService("udevd")
	udevd.TypedSpec().Running = true
	udevd.TypedSpec().Healthy = true
	suite.Create(udevd)

	createVolumeStatus(&suite.DefaultSuite, constants.MetaPartitionLabel, block.VolumePhaseReady, block.EncryptionProviderNone, "/dev/sda4", "/dev/sda4")

	suite.createLVMPV("sdb", "/dev/sdb")
	suite.createLVMPV("dm-0", "/dev/dm-0")

	// Nothing is activated until the machine config is known.
	time.Sleep(250 * time.Millisecond)
	suite.Assert().Zero(suite.activator.activationCount("foreign"))

	applyMachineConfig(&suite.DefaultSuite, newParentsVGDoc("vg-pool", "lvm"))

	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		if suite.activator.activationCount("foreign") == 0 {
			return retry.ExpectedErrorf("foreign VG not activated yet")
		}

		return nil
	})

	suite.Assert().Zero(suite.activator.activationCount("vg-pool"))
}

func TestLVMActivationSuite(t *testing.T) {
	t.Parallel()

	activator := &fakeActivator{
		vgByDevice: map[string]string{
			"/dev/sdb":  "foreign",
			"/dev/dm-0": "vg-pool",
		},
		activated: map[string]int{},
	}

	suite.Run(t, &LVMActivationSuite{
		activator: activator,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&storagectrl.LVMActivationController{
					LVM: activator,
				}))
			},
		},
	})
}
