// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

type NodeIdentitySuite struct {
	ctest.DefaultSuite
}

func (suite *NodeIdentitySuite) TestDefault() {
	statePath := suite.T().TempDir()
	mountID := (&clusterctrl.NodeIdentityController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	ctest.AssertNoResource[*cluster.Identity](suite, cluster.LocalIdentity)

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertResource(suite, cluster.LocalIdentity, func(*cluster.Identity, *assert.Assertions) {})
	ctest.AssertResource(suite, "machine-id", func(*files.EtcFileSpec, *assert.Assertions) {})

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)
}

func (suite *NodeIdentitySuite) TestLoad() {
	statePath := suite.T().TempDir()
	mountID := (&clusterctrl.NodeIdentityController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	// using verbatim data here to make sure nodeId representation is supported in future version fo Talos
	suite.Require().NoError(os.WriteFile(filepath.Join(statePath, constants.NodeIdentityFilename), []byte("nodeId: gvqfS27LxD58lPlASmpaueeRVzuof16iXoieRgEvBWaE\n"), 0o600))

	ctest.AssertNoResource[*cluster.Identity](suite, cluster.LocalIdentity)

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertResource(suite, cluster.LocalIdentity, func(identity *cluster.Identity, asrt *assert.Assertions) {
		asrt.Equal("gvqfS27LxD58lPlASmpaueeRVzuof16iXoieRgEvBWaE", identity.TypedSpec().NodeID)
	})
	ctest.AssertResource(suite, "machine-id", func(f *files.EtcFileSpec, asrt *assert.Assertions) {
		asrt.Equal("8d2c0de2408fa2a178bad7f45d9aa8fb", string(f.TypedSpec().Contents))
	})

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)
}

func TestNodeIdentitySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &NodeIdentitySuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&clusterctrl.NodeIdentityController{}))
			},
		},
	})
}
