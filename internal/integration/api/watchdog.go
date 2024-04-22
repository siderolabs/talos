// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// WatchdogSuite ...
type WatchdogSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *WatchdogSuite) SuiteName() string {
	return "api.WatchdogSuite"
}

// SetupTest ...
func (suite *WatchdogSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)

	if suite.Cluster == nil || suite.Cluster.Provisioner() != "qemu" {
		suite.T().Skip("skipping watchdog test since provisioner is not qemu")
	}
}

// TearDownTest ...
func (suite *WatchdogSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *WatchdogSuite) readWatchdogSysfs(nodeCtx context.Context, watchdog, property string) string { //nolint:unparam
	r, err := suite.Client.Read(nodeCtx, filepath.Join("/sys/class/watchdog", watchdog, property))
	suite.Require().NoError(err)

	value, err := io.ReadAll(r)
	suite.Require().NoError(err)

	suite.Require().NoError(r.Close())

	return string(bytes.TrimSpace(value))
}

// TestWatchdogSysfs sets up the watchdog and validates its parameters from the /sys/class/watchdog.
func (suite *WatchdogSuite) TestWatchdogSysfs() {
	// pick up a random node to test the watchdog on, and use it throughout the test
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Logf("testing watchdog on node %s", node)

	// build a Talos API context which is tied to the node
	nodeCtx := client.WithNode(suite.ctx, node)

	// pick a watchdog
	const watchdog = "watchdog0"

	cfgDocument := runtime.NewWatchdogTimerV1Alpha1()
	cfgDocument.WatchdogDevice = "/dev/" + watchdog
	cfgDocument.WatchdogTimeout = 120 * time.Second

	// deactivate the watchdog
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	_, err := suite.Client.COSI.WatchFor(nodeCtx, runtimeres.NewWatchdogTimerStatus(runtimeres.WatchdogTimerConfigID).Metadata(), state.WithEventTypes(state.Destroyed))
	suite.Require().NoError(err)

	wdState := suite.readWatchdogSysfs(nodeCtx, watchdog, "state")
	suite.Require().Equal("inactive", wdState)

	// enable watchdog with 120s timeout
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, runtimeres.NewWatchdogTimerStatus(runtimeres.WatchdogTimerConfigID).Metadata(), state.WithEventTypes(state.Created, state.Updated))
	suite.Require().NoError(err)

	wdState = suite.readWatchdogSysfs(nodeCtx, watchdog, "state")
	suite.Require().Equal("active", wdState)

	wdTimeout := suite.readWatchdogSysfs(nodeCtx, watchdog, "timeout")
	suite.Require().Equal("120", wdTimeout)

	// update watchdog timeout to 60s
	cfgDocument.WatchdogTimeout = 60 * time.Second
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, runtimeres.NewWatchdogTimerStatus(runtimeres.WatchdogTimerConfigID).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			return r.(*runtimeres.WatchdogTimerStatus).TypedSpec().Timeout == cfgDocument.WatchdogTimeout, nil
		}),
	)
	suite.Require().NoError(err)

	wdState = suite.readWatchdogSysfs(nodeCtx, watchdog, "state")
	suite.Require().Equal("active", wdState)

	wdTimeout = suite.readWatchdogSysfs(nodeCtx, watchdog, "timeout")
	suite.Require().Equal("60", wdTimeout)

	// deactivate the watchdog
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, runtimeres.NewWatchdogTimerStatus(runtimeres.WatchdogTimerConfigID).Metadata(), state.WithEventTypes(state.Destroyed))
	suite.Require().NoError(err)

	wdState = suite.readWatchdogSysfs(nodeCtx, watchdog, "state")
	suite.Require().Equal("inactive", wdState)
}

func init() {
	allSuites = append(allSuites, new(WatchdogSuite))
}
