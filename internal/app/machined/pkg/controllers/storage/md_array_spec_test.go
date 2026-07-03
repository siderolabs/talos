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

type MDArraySpecSuite struct {
	ctest.DefaultSuite
}

func (suite *MDArraySpecSuite) TestRendersSpecFromDoc() {
	applyMachineConfigDocs(&suite.DefaultSuite, newRAIDDoc("data", `disk.transport == "nvme"`))

	ctest.AssertResource(suite, "data", func(spec *storageres.MDArraySpec, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDLevelRAID1, spec.TypedSpec().Level)
		asrt.False(spec.TypedSpec().VolumeSelector.IsZero())
	})
}

func (suite *MDArraySpecSuite) TestRendersMultipleDocs() {
	applyMachineConfigDocs(
		&suite.DefaultSuite,
		newRAIDDoc("data", `disk.transport == "nvme"`),
		newRAIDDoc("logs", `disk.transport == "sata"`),
	)

	ctest.AssertResource(suite, "data", func(*storageres.MDArraySpec, *assert.Assertions) {})
	ctest.AssertResource(suite, "logs", func(*storageres.MDArraySpec, *assert.Assertions) {})
}

func (suite *MDArraySpecSuite) TestRemovingDocRemovesSpec() {
	cfg := applyMachineConfigDocs(&suite.DefaultSuite, newRAIDDoc("temp", `disk.transport == "nvme"`))

	ctest.AssertResource(suite, "temp", func(*storageres.MDArraySpec, *assert.Assertions) {})

	suite.Destroy(cfg)

	ctest.AssertNoResource[*storageres.MDArraySpec](suite, "temp")
}

func TestMDArraySpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &MDArraySpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&storagectrl.MDArraySpecController{}))
			},
		},
	})
}
