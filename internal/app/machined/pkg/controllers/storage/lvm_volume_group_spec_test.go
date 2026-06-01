// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

type LVMVolumeGroupSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *LVMVolumeGroupSpecSuite) createPVSpec(id, device, vgName string) {
	pv := storageres.NewLVMPhysicalVolumeSpec(storageres.NamespaceName, id)
	pv.TypedSpec().Device = device
	pv.TypedSpec().VGName = vgName

	suite.Create(pv)
}

func (suite *LVMVolumeGroupSpecSuite) TestAggregatesPVsByVG() {
	suite.createPVSpec("nvme0n1", "/dev/nvme0n1", "vg-pool")
	suite.createPVSpec("nvme1n1", "/dev/nvme1n1", "vg-pool")
	suite.createPVSpec("sda", "/dev/sda", "vg-other")

	applyMachineConfig(
		&suite.DefaultSuite,
		newVGDoc("vg-pool", `disk.transport == "nvme"`),
		newVGDoc("vg-other", `disk.transport == "sata"`),
	)

	ctest.AssertResource(suite, "vg-pool", func(vg *storageres.LVMVolumeGroupSpec, asrt *assert.Assertions) {
		asrt.Equal("vg-pool", vg.TypedSpec().Name)
		asrt.Equal([]string{"/dev/nvme0n1", "/dev/nvme1n1"}, vg.TypedSpec().PhysicalVolumes)
	})

	ctest.AssertResource(suite, "vg-other", func(vg *storageres.LVMVolumeGroupSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"/dev/sda"}, vg.TypedSpec().PhysicalVolumes)
	})
}

func (suite *LVMVolumeGroupSpecSuite) TestEmitsSpecWithoutPVsYet() {
	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-empty", `disk.transport == "nvme"`))

	ctest.AssertResource(suite, "vg-empty", func(vg *storageres.LVMVolumeGroupSpec, asrt *assert.Assertions) {
		asrt.Empty(vg.TypedSpec().PhysicalVolumes)
	})
}

func (suite *LVMVolumeGroupSpecSuite) TestRemovingDocRemovesSpec() {
	cfg := applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-temp", `disk.transport == "nvme"`))

	ctest.AssertResource(suite, "vg-temp", func(*storageres.LVMVolumeGroupSpec, *assert.Assertions) {})

	suite.Destroy(cfg)

	ctest.AssertNoResource[*storageres.LVMVolumeGroupSpec](suite, "vg-temp")
}

func TestLVMVolumeGroupSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LVMVolumeGroupSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&storagectrl.LVMVolumeGroupSpecController{}))
			},
		},
	})
}
