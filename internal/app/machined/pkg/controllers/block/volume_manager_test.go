// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type VolumeManagerSuite struct {
	ctest.DefaultSuite
}

func TestVolumeManagerSuite(t *testing.T) {
	suite.Run(t, &VolumeManagerSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.VolumeManagerController{}))
			},
		},
	})
}

// setupFailingVolume creates a VolumeLifecycle and a volume config that references a
// non-existent disk (so it can never be provisioned), waits for the volume to end up
// in the Failed phase, and pins a finalizer on its status to simulate a consumer that
// is never cleaned up. It returns the created VolumeLifecycle.
func (suite *VolumeManagerSuite) setupFailingVolume(id string, labels ...string) *block.VolumeLifecycle {
	ctx := suite.Ctx()

	// discovered volumes are ready
	discoveredVolumesStatus := block.NewDiscoveredVolumesStatus(block.NamespaceName, block.DiscoveredVolumesStatusID)
	discoveredVolumesStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.State().Create(ctx, discoveredVolumesStatus))

	// volume lifecycle exists (controller ceases all ops without it)
	lifecycle := block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)
	suite.Require().NoError(suite.State().Create(ctx, lifecycle))

	createVolumeOnMissingDisk(&suite.DefaultSuite, id, labels...)

	// the volume should end up Failed (no disk matched selector)
	ctest.AssertResource(suite, id, func(vs *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(block.VolumePhaseFailed, vs.TypedSpec().Phase)
	})

	// pin a finalizer on the failing volume status to simulate a consumer that is
	// never cleaned up, so the volume cannot be closed the usual way.
	suite.AddFinalizer(block.NewVolumeStatus(block.NamespaceName, id).Metadata(), "test")
	ctest.AssertResource(suite, id, func(vs *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.True(vs.Metadata().Finalizers().Has("test"))
	})

	return lifecycle
}

// createVolumeOnMissingDisk creates a partition volume config referencing a non-existent disk,
// so that it can never be provisioned.
func createVolumeOnMissingDisk(suite *ctest.DefaultSuite, id string, labels ...string) {
	vc := block.NewVolumeConfig(block.NamespaceName, id)
	for _, label := range labels {
		vc.Metadata().Labels().Set(label, "")
	}

	vc.TypedSpec().Type = block.VolumeTypePartition
	vc.TypedSpec().Provisioning = block.ProvisioningSpec{
		Wave: block.WaveUserVolumes,
		DiskSelector: block.DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.dev_path == "/dev/does-not-exist"`, celenv.DiskLocator())),
		},
		PartitionSpec: block.PartitionSpec{
			MinSize: 1024 * 1024,
		},
	}
	vc.TypedSpec().Locator = block.LocatorSpec{
		Match: cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_label == "`+id+`"`, celenv.VolumeLocator())),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vc))
}

// assertFailingVolumeDoesNotBlockReset sets up a failing volume with the given label,
// tears down the VolumeLifecycle (as reset does), and asserts the controller releases
// its lifecycle finalizer so the teardown can complete.
func (suite *VolumeManagerSuite) assertFailingVolumeDoesNotBlockReset(label string) {
	lifecycle := suite.setupFailingVolume("v-bad", label)

	// simulate reset: tear down the VolumeLifecycle
	_, err := suite.State().Teardown(suite.Ctx(), lifecycle.Metadata())
	suite.Require().NoError(err)

	// the controller must remove its finalizer so TeardownVolumeLifecycle can
	// proceed; otherwise reset is blocked by the failing volume.
	ctest.AssertResource(suite, block.VolumeLifecycleID, func(lc *block.VolumeLifecycle, asrt *assert.Assertions) {
		asrt.True(lc.Metadata().Finalizers().Empty(), "finalizers: %v", lc.Metadata().Finalizers())
	})
}

// A user-configured volume whose allocation fails (references a non-existent disk) must
// not hold up a global volume lifecycle teardown (reset/reboot/upgrade), even if its
// status still has a finalizer. This applies to every user-configured volume kind.

func (suite *VolumeManagerSuite) TestReadyExternalVolumePropagatesMountSpecUpdates() {
	ctx := suite.Ctx()

	discoveredVolumesStatus := block.NewDiscoveredVolumesStatus(block.NamespaceName, block.DiscoveredVolumesStatusID)
	discoveredVolumesStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.State().Create(ctx, discoveredVolumesStatus))

	lifecycle := block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)
	suite.Require().NoError(suite.State().Create(ctx, lifecycle))

	volumeConfig := block.NewVolumeConfig(block.NamespaceName, "external-nfs")
	volumeConfig.TypedSpec().Type = block.VolumeTypeExternal
	volumeConfig.TypedSpec().Provisioning.DiskSelector.External = "192.0.2.10:/export"
	volumeConfig.TypedSpec().Provisioning.FilesystemSpec.Type = block.FilesystemTypeNFS
	volumeConfig.TypedSpec().Mount.Parameters = []block.ParameterSpec{
		block.NewStringParameter("vers", "3"),
		block.NewBooleanParameter("nolock"),
	}
	suite.Require().NoError(suite.State().Create(ctx, volumeConfig))

	ctest.AssertResource(suite, "external-nfs", func(status *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
		asrt.Equal(volumeConfig.TypedSpec().Mount, status.TypedSpec().MountSpec)
	})

	volumeConfig, err := safe.StateGetByID[*block.VolumeConfig](ctx, suite.State(), "external-nfs")
	suite.Require().NoError(err)

	volumeConfig.TypedSpec().Mount.Parameters = []block.ParameterSpec{
		block.NewStringParameter("vers", "4.1"),
	}
	suite.Update(volumeConfig)

	ctest.AssertResource(suite, "external-nfs", func(status *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
		asrt.Equal(volumeConfig.TypedSpec().Mount, status.TypedSpec().MountSpec)
	})
}

func (suite *VolumeManagerSuite) TestFailingUserVolumeDoesNotBlockReset() {
	suite.assertFailingVolumeDoesNotBlockReset(block.UserVolumeLabel)
}

func (suite *VolumeManagerSuite) TestFailingRawVolumeDoesNotBlockReset() {
	suite.assertFailingVolumeDoesNotBlockReset(block.RawVolumeLabel)
}

func (suite *VolumeManagerSuite) TestFailingExistingVolumeDoesNotBlockReset() {
	suite.assertFailingVolumeDoesNotBlockReset(block.ExistingVolumeLabel)
}

func (suite *VolumeManagerSuite) TestFailingSwapVolumeDoesNotBlockReset() {
	suite.assertFailingVolumeDoesNotBlockReset(block.SwapVolumeLabel)
}

// TestFailingSystemVolumeStillBlocks verifies the fix is scoped to user-configured
// volumes: a failing system volume with a lingering finalizer must still hold the
// lifecycle teardown (it may need a real close/unmount), so the finalizer stays.
func (suite *VolumeManagerSuite) TestFailingSystemVolumeStillBlocks() {
	lifecycle := suite.setupFailingVolume("s-bad", block.SystemVolumeLabel)

	_, err := suite.State().Teardown(suite.Ctx(), lifecycle.Metadata())
	suite.Require().NoError(err)

	// Capture ctx/state in locals: Never leaks its last condition goroutine past
	// return, and reading suite.Ctx()/suite.State() there races the next test's
	// SetupTest overwriting those fields.
	ctx, st := suite.Ctx(), suite.State()

	// lifecycle finalizer must NOT be removed while the system volume is not closed
	suite.Assert().Never(func() bool {
		lc, err := safe.StateGetByID[*block.VolumeLifecycle](ctx, st, block.VolumeLifecycleID)
		if err != nil {
			return false
		}

		return lc.Metadata().Phase() == resource.PhaseTearingDown && lc.Metadata().Finalizers().Empty()
	}, 2*time.Second, 200*time.Millisecond)
}

const testBootPartitionUUID = "6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001"

// setupBootPartitionWait publishes a boot partition which is not discovered (yet), marks the devices
// ready and creates a volume which can never be provisioned: without the boot partition wait the volume
// would immediately fail (which is what happens to META/STATE being declared missing on a real machine).
func setupBootPartitionWait(suite *ctest.DefaultSuite, id string) {
	ctx := suite.Ctx()

	bootPartition := runtimeres.NewBootPartitionStatus(runtimeres.NamespaceName, runtimeres.BootPartitionStatusID)
	bootPartition.TypedSpec().PartitionUUID = testBootPartitionUUID
	suite.Require().NoError(suite.State().Create(ctx, bootPartition))

	discoveredVolumesStatus := block.NewDiscoveredVolumesStatus(block.NamespaceName, block.DiscoveredVolumesStatusID)
	discoveredVolumesStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.State().Create(ctx, discoveredVolumesStatus))

	lifecycle := block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)
	suite.Require().NoError(suite.State().Create(ctx, lifecycle))

	createVolumeOnMissingDisk(suite, id)
}

type VolumeManagerBootPartitionSuite struct {
	ctest.DefaultSuite
}

func TestVolumeManagerBootPartitionSuite(t *testing.T) {
	suite.Run(t, &VolumeManagerBootPartitionSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.VolumeManagerController{
					BootPartitionWaitTimeout: 10 * time.Second,
				}))
			},
		},
	})
}

// The volumes are kept waiting until the boot partition is discovered, so that a slow boot disk
// doesn't get its volumes declared missing.
func (suite *VolumeManagerBootPartitionSuite) TestWaitsForBootPartition() {
	setupBootPartitionWait(&suite.DefaultSuite, "v-boot")

	ctest.AssertResource(suite, "v-boot", func(vs *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(block.VolumePhaseWaiting, vs.TypedSpec().Phase)
	})

	// still waiting a bit later
	time.Sleep(time.Second)

	ctest.AssertResource(suite, "v-boot", func(vs *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(block.VolumePhaseWaiting, vs.TypedSpec().Phase)
	})

	// the boot partition shows up (matched case-insensitively): the volume proceeds, and fails as its disk doesn't exist
	dv := block.NewDiscoveredVolume(block.NamespaceName, "sda1")
	dv.TypedSpec().Type = "partition"
	dv.TypedSpec().PartitionUUID = strings.ToUpper(testBootPartitionUUID)
	suite.Create(dv)

	ctest.AssertResource(suite, "v-boot", func(vs *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(block.VolumePhaseFailed, vs.TypedSpec().Phase)
	})
}

type VolumeManagerBootPartitionTimeoutSuite struct {
	ctest.DefaultSuite
}

func TestVolumeManagerBootPartitionTimeoutSuite(t *testing.T) {
	suite.Run(t, &VolumeManagerBootPartitionTimeoutSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.VolumeManagerController{
					BootPartitionWaitTimeout: time.Second,
				}))
			},
		},
	})
}

// The wait for the boot partition is bounded: if it never shows up, the volumes proceed after the timeout.
func (suite *VolumeManagerBootPartitionTimeoutSuite) TestBootPartitionWaitTimesOut() {
	setupBootPartitionWait(&suite.DefaultSuite, "v-boot")

	ctest.AssertResource(suite, "v-boot", func(vs *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(block.VolumePhaseFailed, vs.TypedSpec().Phase)
	})
}
