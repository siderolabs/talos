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

type VolumeTrimScheduleSuite struct {
	ctest.DefaultSuite
}

const (
	testTrimInterval = time.Hour
	testTrimNodeID   = "test-node-id"
)

func TestVolumeTrimScheduleSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VolumeTrimScheduleSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.VolumeTrimScheduleController{}))
			},
		},
	})
}

func (suite *VolumeTrimScheduleSuite) createIdentity() {
	identity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	identity.TypedSpec().NodeID = testTrimNodeID
	suite.Create(identity)
}

func (suite *VolumeTrimScheduleSuite) createVolumeStatus(id string, mutate func(*block.VolumeStatusSpec)) {
	volumeStatus := block.NewVolumeStatus(block.NamespaceName, id)
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	volumeStatus.TypedSpec().Type = block.VolumeTypePartition
	volumeStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
	volumeStatus.TypedSpec().TrimEnabled = true
	volumeStatus.TypedSpec().TrimInterval = testTrimInterval

	if mutate != nil {
		mutate(volumeStatus.TypedSpec())
	}

	suite.Create(volumeStatus)
}

func (suite *VolumeTrimScheduleSuite) TestEligibility() {
	suite.createIdentity()

	// eligible: ready partition with xfs, trim enabled
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, nil)

	// not eligible: trim disabled
	suite.createVolumeStatus("trim-disabled", func(spec *block.VolumeStatusSpec) {
		spec.TrimEnabled = false
	})

	// not eligible: vfat does not support trim
	suite.createVolumeStatus("vfat", func(spec *block.VolumeStatusSpec) {
		spec.Filesystem = block.FilesystemTypeVFAT
	})

	// not eligible: not ready
	suite.createVolumeStatus("not-ready", func(spec *block.VolumeStatusSpec) {
		spec.Phase = block.VolumePhasePrepared
	})

	now := time.Now()

	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(schedule *block.VolumeTrimSchedule, asrt *assert.Assertions) {
		asrt.Equal(testTrimInterval, schedule.TypedSpec().Interval)
		asrt.True(schedule.TypedSpec().NextTrim.After(now), "next trim should be in the future")
		asrt.True(schedule.TypedSpec().NextTrim.Before(now.Add(testTrimInterval).Add(time.Second)), "next trim should be within one interval")
	})

	ctest.AssertNoResource[*block.VolumeTrimSchedule](suite, "trim-disabled")
	ctest.AssertNoResource[*block.VolumeTrimSchedule](suite, "vfat")
	ctest.AssertNoResource[*block.VolumeTrimSchedule](suite, "not-ready")
}

func (suite *VolumeTrimScheduleSuite) TestNoIdentityNoSchedule() {
	// no node identity yet -> no schedule is created.
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, nil)

	ctest.AssertNoResource[*block.VolumeTrimSchedule](suite, constants.EphemeralPartitionLabel)
}

func (suite *VolumeTrimScheduleSuite) TestEncryptedAllowDiscards() {
	suite.createIdentity()

	// encrypted volume with discards allowed -> eligible
	suite.createVolumeStatus("encrypted-discards", func(spec *block.VolumeStatusSpec) {
		spec.EncryptionProvider = block.EncryptionProviderLUKS2
		spec.EncryptionAllowDiscards = true
	})

	// encrypted volume with discards not allowed -> not eligible
	suite.createVolumeStatus("encrypted-nodiscards", func(spec *block.VolumeStatusSpec) {
		spec.EncryptionProvider = block.EncryptionProviderLUKS2
		spec.EncryptionAllowDiscards = false
	})

	ctest.AssertResource(suite, "encrypted-discards", func(*block.VolumeTrimSchedule, *assert.Assertions) {})
	ctest.AssertNoResource[*block.VolumeTrimSchedule](suite, "encrypted-nodiscards")
}

func (suite *VolumeTrimScheduleSuite) TestCleanup() {
	suite.createIdentity()
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, nil)

	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(*block.VolumeTrimSchedule, *assert.Assertions) {})

	// disable trimming -> schedule should be cleaned up.
	_, err := suite.State().UpdateWithConflicts(suite.Ctx(), block.NewVolumeStatus(block.NamespaceName, constants.EphemeralPartitionLabel).Metadata(), func(r resource.Resource) error {
		r.(*block.VolumeStatus).TypedSpec().TrimEnabled = false

		return nil
	})
	suite.Require().NoError(err)

	ctest.AssertNoResource[*block.VolumeTrimSchedule](suite, constants.EphemeralPartitionLabel)
}
