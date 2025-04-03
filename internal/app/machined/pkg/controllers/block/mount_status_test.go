// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type MountStatusSuite struct {
	ctest.DefaultSuite
}

func TestMountStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &MountStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.MountStatusController{}))
			},
		},
	})
}

func (suite *MountStatusSuite) TestReconcile() {
	mountStatus1 := block.NewMountStatus(block.NamespaceName, "volume1")
	mountStatus1.TypedSpec().Spec = block.MountRequestSpec{
		VolumeID:     "volume1",
		Requesters:   []string{"requester1", "requester2"},
		RequesterIDs: []string{"requester1/volume1", "requester2/volume1"},
	}
	mountStatus1.TypedSpec().Target = "/target"
	suite.Create(mountStatus1)

	// mount status is exploded into volume mount statuses
	ctest.AssertResources(suite,
		[]resource.ID{"requester1/volume1", "requester2/volume1"},
		func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.Equal("volume1", vms.Metadata().Labels().Raw()["mount-status-id"])
			asrt.Equal("volume1", vms.TypedSpec().VolumeID)
			asrt.Equal("/target", vms.TypedSpec().Target)
		},
	)

	// mount status should now have a finalizer
	ctest.AssertResource(suite, "volume1", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has((&blockctrls.MountStatusController{}).Name()))
	})

	// add a finalizer for volume mount status
	suite.AddFinalizer(block.NewVolumeMountStatus(block.NamespaceName, "requester1/volume1").Metadata(), "test-finalizer")

	// now, teardown the mount status
	ready, err := suite.State().Teardown(suite.Ctx(), mountStatus1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().False(ready)

	// volume mount status without finalizer should be removed
	ctest.AssertNoResource[*block.VolumeMountStatus](suite, "requester2/volume1")

	// volume mount status with finalizer should be tearing down
	ctest.AssertResource(suite, "requester1/volume1", func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, vms.Metadata().Phase())
	})

	// remove finalizer from volume mount status
	suite.RemoveFinalizer(block.NewVolumeMountStatus(block.NamespaceName, "requester1/volume1").Metadata(), "test-finalizer")

	// volume mount status should be destroyed
	ctest.AssertNoResource[*block.VolumeMountStatus](suite, "requester1/volume1")

	// now the mount status finalizers should be empty as well
	ctest.AssertResource(suite, "volume1", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(mountStatus1)
}

func (suite *MountStatusSuite) TestReconcileRequesterGoingOut() {
	mountStatus1 := block.NewMountStatus(block.NamespaceName, "volume1")
	mountStatus1.TypedSpec().Spec = block.MountRequestSpec{
		VolumeID:     "volume1",
		Requesters:   []string{"requester1", "requester2"},
		RequesterIDs: []string{"requester1/volume1", "requester2/volume1"},
	}
	mountStatus1.TypedSpec().Target = "/target"
	suite.Create(mountStatus1)

	// mount status is exploded into volume mount statuses
	ctest.AssertResources(suite,
		[]resource.ID{"requester1/volume1", "requester2/volume1"},
		func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.Equal("volume1", vms.Metadata().Labels().Raw()["mount-status-id"])
			asrt.Equal("volume1", vms.TypedSpec().VolumeID)
			asrt.Equal("/target", vms.TypedSpec().Target)
		},
	)

	// put a finalizer on volume mount status
	suite.AddFinalizer(block.NewVolumeMountStatus(block.NamespaceName, "requester1/volume1").Metadata(), "test-finalizer")

	// update the mount status, as if requester1 is no longer mounting it
	mountStatus1, err := safe.StateGetByID[*block.MountStatus](suite.Ctx(), suite.State(), mountStatus1.Metadata().ID())
	suite.Require().NoError(err)

	mountStatus1.TypedSpec().Spec.Requesters = []string{"requester2"}
	mountStatus1.TypedSpec().Spec.RequesterIDs = []string{"requester2/volume1"}
	suite.Update(mountStatus1)

	// volume mount status with finalizer should be tearing down
	ctest.AssertResource(suite, "requester1/volume1", func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, vms.Metadata().Phase())
	})

	// remove finalizer from volume mount status
	suite.RemoveFinalizer(block.NewVolumeMountStatus(block.NamespaceName, "requester1/volume1").Metadata(), "test-finalizer")

	// volume mount status should be destroyed
	ctest.AssertNoResource[*block.VolumeMountStatus](suite, "requester1/volume1")
}
