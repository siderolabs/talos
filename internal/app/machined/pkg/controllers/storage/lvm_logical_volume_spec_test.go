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

type LVMLogicalVolumeSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *LVMLogicalVolumeSpecSuite) TestEmitsSpecFromDoc() {
	applyMachineConfigDocs(&suite.DefaultSuite, newLVDoc("lv-data", "vg-pool", storageres.LVMLogicalVolumeTypeLinear, "50GiB"))

	ctest.AssertResource(suite, "vg-pool-lv-data", func(lv *storageres.LVMLogicalVolumeSpec, asrt *assert.Assertions) {
		spec := lv.TypedSpec()
		asrt.Equal("vg-pool", spec.VGName)
		asrt.Equal("lv-data", spec.Name)
		asrt.Equal(storageres.LVMLogicalVolumeTypeLinear, spec.Type)
		asrt.Equal(uint64(50*1024*1024*1024), spec.SizeBytes)
		asrt.Equal(uint32(0), spec.SizePercentVG)
	})
}

func (suite *LVMLogicalVolumeSpecSuite) TestResolvesPercentSize() {
	applyMachineConfigDocs(&suite.DefaultSuite, newLVDoc("lv-pct", "vg-pool", storageres.LVMLogicalVolumeTypeRAID1, "80%"))

	ctest.AssertResource(suite, "vg-pool-lv-pct", func(lv *storageres.LVMLogicalVolumeSpec, asrt *assert.Assertions) {
		spec := lv.TypedSpec()
		asrt.Equal(storageres.LVMLogicalVolumeTypeRAID1, spec.Type)
		asrt.Equal(uint64(0), spec.SizeBytes)
		asrt.Equal(uint32(80), spec.SizePercentVG)
		asrt.Equal(uint32(1), spec.Mirrors) // raid1 default mirrors
		asrt.Equal(uint32(0), spec.Stripes)
	})
}

func (suite *LVMLogicalVolumeSpecSuite) TestRemovingDocCleansSpec() {
	cfg := applyMachineConfigDocs(&suite.DefaultSuite, newLVDoc("lv-data", "vg-pool", storageres.LVMLogicalVolumeTypeLinear, "50GiB"))

	ctest.AssertResource(suite, "vg-pool-lv-data", func(*storageres.LVMLogicalVolumeSpec, *assert.Assertions) {})

	suite.Destroy(cfg)

	ctest.AssertNoResource[*storageres.LVMLogicalVolumeSpec](suite, "vg-pool-lv-data")
}

func TestLVMLogicalVolumeSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LVMLogicalVolumeSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&storagectrl.LVMLogicalVolumeSpecController{}))
			},
		},
	})
}
