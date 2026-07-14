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

	// devices are ready
	devicesStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
	devicesStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.State().Create(ctx, devicesStatus))

	// volume lifecycle exists (controller ceases all ops without it)
	lifecycle := block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)
	suite.Require().NoError(suite.State().Create(ctx, lifecycle))

	// volume referencing a non-existent disk
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
	suite.Require().NoError(suite.State().Create(ctx, vc))

	// the controller writes a DiscoveryRefreshRequest once devices become ready;
	// mirror it back as a DiscoveryRefreshStatus so devicesReady becomes true.
	suite.Assert().Eventually(func() bool {
		req, err := safe.StateGetByID[*block.DiscoveryRefreshRequest](ctx, suite.State(), block.RefreshID)
		if err != nil {
			return false
		}

		st := block.NewDiscoveryRefreshStatus(block.NamespaceName, block.RefreshID)
		st.TypedSpec().Request = req.TypedSpec().Request

		if err := suite.State().Create(ctx, st); err != nil {
			// already created - update
			existing, gerr := safe.StateGetByID[*block.DiscoveryRefreshStatus](ctx, suite.State(), block.RefreshID)
			if gerr != nil {
				return false
			}

			existing.TypedSpec().Request = req.TypedSpec().Request
			suite.Require().NoError(suite.State().Update(ctx, existing))
		}

		return true
	}, 10*time.Second, 100*time.Millisecond)

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
