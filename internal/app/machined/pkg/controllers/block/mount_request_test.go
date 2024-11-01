// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type MountRequestSuite struct {
	ctest.DefaultSuite
}

func TestMountRequestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &MountRequestSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.MountRequestController{}))
			},
		},
	})
}

func (suite *MountRequestSuite) TestReconcile() {
	mountRequest1 := block.NewVolumeMountRequest(block.NamespaceName, "mountRequest1")
	mountRequest1.TypedSpec().Requester = "requester1"
	mountRequest1.TypedSpec().VolumeID = "volume1"
	mountRequest1.TypedSpec().ReadOnly = true
	suite.Create(mountRequest1)

	// mount request is not created as the volume is not ready
	ctest.AssertNoResource[*block.MountRequest](suite, "volume1")

	volumeStatus1 := block.NewVolumeStatus(block.NamespaceName, "volume1")
	volumeStatus1.TypedSpec().Phase = block.VolumePhaseWaiting
	suite.Create(volumeStatus1)

	// mount request is not created as the volume status is not ready
	ctest.AssertNoResource[*block.MountRequest](suite, "volume1")

	volumeStatus1.TypedSpec().Phase = block.VolumePhaseReady
	suite.Update(volumeStatus1)

	ctest.AssertResource(suite, "volume1", func(mr *block.MountRequest, asrt *assert.Assertions) {
		asrt.Equal("volume1", mr.TypedSpec().VolumeID)
		asrt.True(mr.TypedSpec().ReadOnly)
		asrt.ElementsMatch([]string{"requester1"}, mr.TypedSpec().Requesters)
		asrt.ElementsMatch([]string{"mountRequest1"}, mr.TypedSpec().RequesterIDs)
	})

	// add another mount request for the same volume
	mountRequest2 := block.NewVolumeMountRequest(block.NamespaceName, "mountRequest2")
	mountRequest2.TypedSpec().Requester = "requester2"
	mountRequest2.TypedSpec().VolumeID = "volume1"
	mountRequest2.TypedSpec().ReadOnly = false
	suite.Create(mountRequest2)

	ctest.AssertResource(suite, "volume1", func(mr *block.MountRequest, asrt *assert.Assertions) {
		asrt.Equal("volume1", mr.TypedSpec().VolumeID)
		asrt.False(mr.TypedSpec().ReadOnly)
		asrt.ElementsMatch([]string{"requester1", "requester2"}, mr.TypedSpec().Requesters)
		asrt.ElementsMatch([]string{"mountRequest1", "mountRequest2"}, mr.TypedSpec().RequesterIDs)
	})

	// if the mount request is fulfilled, a finalizer should be added
	suite.AddFinalizer(block.NewMountRequest(block.NamespaceName, "volume1").Metadata(), "mounted")

	// try to remove one mount requests now
	suite.Destroy(mountRequest2)

	ctest.AssertResource(suite, "volume1", func(mr *block.MountRequest, asrt *assert.Assertions) {
		asrt.Equal("volume1", mr.TypedSpec().VolumeID)
		asrt.True(mr.TypedSpec().ReadOnly)
		asrt.ElementsMatch([]string{"requester1"}, mr.TypedSpec().Requesters)
		asrt.ElementsMatch([]string{"mountRequest1"}, mr.TypedSpec().RequesterIDs)
	})

	// try to remove another mount request now
	suite.Destroy(mountRequest1)

	ctest.AssertResource(suite, "volume1", func(mr *block.MountRequest, asrt *assert.Assertions) {
		asrt.Equal("volume1", mr.TypedSpec().VolumeID)
		asrt.True(mr.TypedSpec().ReadOnly)
		asrt.Equal([]string{"requester1"}, mr.TypedSpec().Requesters)
		asrt.ElementsMatch([]string{"mountRequest1"}, mr.TypedSpec().RequesterIDs)
		asrt.Equal(resource.PhaseTearingDown, mr.Metadata().Phase())
	})

	// remove the finalizer, allowing the mount request to be destroyed
	suite.RemoveFinalizer(block.NewMountRequest(block.NamespaceName, "volume1").Metadata(), "mounted")

	ctest.AssertNoResource[*block.MountRequest](suite, "volume1")
}
