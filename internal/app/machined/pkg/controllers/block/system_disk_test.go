// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type SystemDiskSuite struct {
	ctest.DefaultSuite
}

func TestSystemDiskSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SystemDiskSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.SystemDiskController{}))
			},
		},
	})
}

func (suite *SystemDiskSuite) TestReconcile() {
	ctest.AssertNoResource[*block.SystemDisk](suite, block.SystemDiskID)

	discoveredVolume := block.NewDiscoveredVolume(block.NamespaceName, "vda4")
	discoveredVolume.TypedSpec().PartitionLabel = constants.MetaPartitionLabel
	discoveredVolume.TypedSpec().Parent = "vda"
	discoveredVolume.TypedSpec().ParentDevPath = "/dev/vda"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), discoveredVolume))

	ctest.AssertResource(suite, block.SystemDiskID, func(r *block.SystemDisk, asrt *assert.Assertions) {
		asrt.Equal("vda", r.TypedSpec().DiskID)
		asrt.Equal("/dev/vda", r.TypedSpec().DevPath)
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), discoveredVolume.Metadata()))

	ctest.AssertNoResource[*block.SystemDisk](suite, block.SystemDiskID)
}
