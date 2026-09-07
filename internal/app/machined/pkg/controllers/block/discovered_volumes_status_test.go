// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type DiscoveredVolumesStatusSuite struct {
	ctest.DefaultSuite
}

func TestDiscoveredVolumesStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &DiscoveredVolumesStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DiscoveredVolumesStatusController{
					WaitForUSB: func(context.Context, *zap.Logger) error { return nil },
				}))
			},
		},
	})
}

func (suite *DiscoveredVolumesStatusSuite) TestNoDevicesStatus() {
	ctest.AssertNoResource[*block.DiscoveredVolumesStatus](suite, block.DiscoveredVolumesStatusID)
	ctest.AssertNoResource[*block.DiscoveryRefreshRequest](suite, block.RefreshID)
}

func (suite *DiscoveredVolumesStatusSuite) TestDevicesNotReady() {
	devicesStatus := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
	devicesStatus.TypedSpec().Ready = false
	suite.Create(devicesStatus)

	ctest.AssertNoResource[*block.DiscoveryRefreshRequest](suite, block.RefreshID)
	ctest.AssertNoResource[*block.DiscoveredVolumesStatus](suite, block.DiscoveredVolumesStatusID)
}

func (suite *DiscoveredVolumesStatusSuite) TestDevicesReadyRequestsRefreshButNotYetDone() {
	devicesStatus := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
	devicesStatus.TypedSpec().Ready = true
	suite.Create(devicesStatus)

	ctest.AssertResource(suite, block.RefreshID, func(r *block.DiscoveryRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Request)
	})

	ctest.AssertNoResource[*block.DiscoveredVolumesStatus](suite, block.DiscoveredVolumesStatusID)
}

func (suite *DiscoveredVolumesStatusSuite) TestDiscoveryRefreshStatusMatchMakesReady() {
	// Setup: create devices ready and observe the refresh request bump.
	devicesStatus := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
	devicesStatus.TypedSpec().Ready = true
	suite.Create(devicesStatus)

	ctest.AssertResource(suite, block.RefreshID, func(r *block.DiscoveryRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Request)
	})

	// Regression test for the missing DiscoveryRefreshStatus input — before the fix, this write alone
	// would never wake the controller, and AssertResource would timeout.
	refreshStatus := block.NewDiscoveryRefreshStatus(block.NamespaceName, block.RefreshID)
	refreshStatus.TypedSpec().Request = 1
	suite.Create(refreshStatus)

	ctest.AssertResource(suite, block.DiscoveredVolumesStatusID, func(r *block.DiscoveredVolumesStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
	})
}

func (suite *DiscoveredVolumesStatusSuite) TestStaleDiscoveryRefreshStatusDoesNotMarkReady() {
	// Setup: create devices ready and observe the refresh request bump.
	devicesStatus := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
	devicesStatus.TypedSpec().Ready = true
	suite.Create(devicesStatus)

	ctest.AssertResource(suite, block.RefreshID, func(r *block.DiscoveryRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Request)
	})

	// Create a stale/mismatched discovery refresh status.
	refreshStatus := block.NewDiscoveryRefreshStatus(block.NamespaceName, block.RefreshID)
	refreshStatus.TypedSpec().Request = 0
	suite.Create(refreshStatus)

	// Give the controller real wall-clock time to react to this write and confirm it
	// correctly declines to mark ready when the status doesn't match.
	ctx, st := suite.Ctx(), suite.State()
	suite.Assert().Never(func() bool {
		_, err := safe.StateGetByID[*block.DiscoveredVolumesStatus](ctx, st, block.DiscoveredVolumesStatusID)

		return err == nil
	}, 1*time.Second, 100*time.Millisecond)
}

func (suite *DiscoveredVolumesStatusSuite) TestReadyDoesNotResetWhenDevicesBecomeNotReady() {
	// Drive the full happy path to get DiscoveredVolumesStatus.Ready = true.
	devicesStatus := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
	devicesStatus.TypedSpec().Ready = true
	suite.Create(devicesStatus)

	ctest.AssertResource(suite, block.RefreshID, func(r *block.DiscoveryRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Request)
	})

	refreshStatus := block.NewDiscoveryRefreshStatus(block.NamespaceName, block.RefreshID)
	refreshStatus.TypedSpec().Request = 1
	suite.Create(refreshStatus)

	ctest.AssertResource(suite, block.DiscoveredVolumesStatusID, func(r *block.DiscoveredVolumesStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
	})

	// Flip devices back to not ready.
	devicesStatus.TypedSpec().Ready = false
	suite.Require().NoError(suite.State().Update(suite.Ctx(), devicesStatus))

	// Confirm Ready remains true (intentional one-way latch behavior).
	ctest.AssertResource(suite, block.DiscoveredVolumesStatusID, func(r *block.DiscoveredVolumesStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
	})
}

// DiscoveredVolumesStatusUSBSuite checks that the discovery refresh is not requested before the USB
// bus is enumerated: a USB-attached system disk shows up long after udevd settles, and if the refresh
// runs without it, the volume manager declares META/STATE missing and the node drops to maintenance.
type DiscoveredVolumesStatusUSBSuite struct {
	ctest.DefaultSuite

	usbSettled chan struct{}
}

func TestDiscoveredVolumesStatusUSBSuite(t *testing.T) {
	t.Parallel()

	s := &DiscoveredVolumesStatusUSBSuite{
		usbSettled: make(chan struct{}),
	}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DiscoveredVolumesStatusController{
				WaitForUSB: func(ctx context.Context, _ *zap.Logger) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-s.usbSettled:
						return nil
					}
				},
			}))
		},
	}

	suite.Run(t, s)
}

func (suite *DiscoveredVolumesStatusUSBSuite) TestWaitsForUSB() {
	devicesStatus := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
	devicesStatus.TypedSpec().Ready = true
	suite.Create(devicesStatus)

	// udevd settled, but the USB bus is still enumerating: no discovery refresh yet
	ctx, st := suite.Ctx(), suite.State()
	suite.Assert().Never(func() bool {
		_, err := safe.StateGetByID[*block.DiscoveryRefreshRequest](ctx, st, block.RefreshID)

		return err == nil
	}, time.Second, 100*time.Millisecond)

	close(suite.usbSettled)

	ctest.AssertResource(suite, block.RefreshID, func(r *block.DiscoveryRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Request)
	})
}
