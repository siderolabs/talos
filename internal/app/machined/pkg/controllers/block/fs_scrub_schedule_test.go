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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

type FSScrubScheduleSuite struct {
	ctest.DefaultSuite
}

const (
	testScrubInterval = time.Hour
	testScrubNodeID   = "test-node-id"
)

func TestFSScrubScheduleSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &FSScrubScheduleSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubScheduleController{}))
			},
		},
	})
}

func (suite *FSScrubScheduleSuite) createIdentity() {
	identity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	identity.TypedSpec().NodeID = testScrubNodeID
	suite.Create(identity)
}

func (suite *FSScrubScheduleSuite) createVolumeStatus(id string, mutate func(*block.VolumeStatusSpec)) {
	volumeStatus := block.NewVolumeStatus(block.NamespaceName, id)
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	volumeStatus.TypedSpec().Type = block.VolumeTypePartition
	volumeStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
	volumeStatus.TypedSpec().ScrubEnabled = true
	volumeStatus.TypedSpec().ScrubInterval = testScrubInterval

	if mutate != nil {
		mutate(volumeStatus.TypedSpec())
	}

	suite.Create(volumeStatus)
}

func (suite *FSScrubScheduleSuite) TestEligibility() {
	suite.createIdentity()

	// eligible: ready xfs volume with scrubbing enabled
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, nil)

	// not eligible: scrubbing disabled
	suite.createVolumeStatus("scrub-disabled", func(spec *block.VolumeStatusSpec) {
		spec.ScrubEnabled = false
	})

	// not eligible: vfat does not support scrubbing
	suite.createVolumeStatus("vfat", func(spec *block.VolumeStatusSpec) {
		spec.Filesystem = block.FilesystemTypeVFAT
	})

	// not eligible: not ready
	suite.createVolumeStatus("not-ready", func(spec *block.VolumeStatusSpec) {
		spec.Phase = block.VolumePhasePrepared
	})

	now := time.Now()

	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(schedule *block.FSScrubSchedule, asrt *assert.Assertions) {
		asrt.Equal(block.FilesystemTypeXFS, schedule.TypedSpec().Filesystem)
		asrt.Equal(testScrubInterval, schedule.TypedSpec().Interval)
		asrt.True(schedule.TypedSpec().NextScrub.After(now), "next scrub should be in the future")
		asrt.True(schedule.TypedSpec().NextScrub.Before(now.Add(testScrubInterval).Add(time.Second)), "next scrub should be within one period")
	})

	ctest.AssertNoResource[*block.FSScrubSchedule](suite, "scrub-disabled")
	ctest.AssertNoResource[*block.FSScrubSchedule](suite, "vfat")
	ctest.AssertNoResource[*block.FSScrubSchedule](suite, "not-ready")
}

func (suite *FSScrubScheduleSuite) TestNoIdentityNoSchedule() {
	// no node identity yet -> no schedule is created.
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, nil)

	ctest.AssertNoResource[*block.FSScrubSchedule](suite, constants.EphemeralPartitionLabel)
}

func (suite *FSScrubScheduleSuite) TestScheduleIsSaltedWithNodeID() {
	suite.createIdentity()
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, nil)

	// the schedule slot is derived from the node ID and the volume ID, so that different
	// nodes of a cluster scrub identically named volumes at different times.
	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(schedule *block.FSScrubSchedule, asrt *assert.Assertions) {
		saltedOffset := block.ScheduleOffset(testScrubNodeID+"/"+constants.EphemeralPartitionLabel, testScrubInterval)

		asrt.Zero(schedule.TypedSpec().NextScrub.Sub(time.Unix(0, int64(saltedOffset))) % testScrubInterval)
	})
}

//nolint:dupl
func (suite *FSScrubScheduleSuite) TestCleanup() {
	suite.createIdentity()
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, nil)

	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(*block.FSScrubSchedule, *assert.Assertions) {})

	// disable scrubbing -> schedule should be cleaned up.
	_, err := suite.State().UpdateWithConflicts(suite.Ctx(), block.NewVolumeStatus(block.NamespaceName, constants.EphemeralPartitionLabel).Metadata(), func(r resource.Resource) error {
		r.(*block.VolumeStatus).TypedSpec().ScrubEnabled = false

		return nil
	})
	suite.Require().NoError(err)

	ctest.AssertNoResource[*block.FSScrubSchedule](suite, constants.EphemeralPartitionLabel)
}
