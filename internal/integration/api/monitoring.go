// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// MonitoringSuite ...
type MonitoringSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *MonitoringSuite) SuiteName() string {
	return "api.MonitoringSuite"
}

// SetupTest ...
func (suite *MonitoringSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)
}

// TearDownTest ...
func (suite *MonitoringSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestMonitoringAPIs tests that monitoring APIs are working.
func (suite *MonitoringSuite) TestMonitoringAPIs() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	_, err := suite.Client.MachineClient.CPUFreqStats(nodeCtx, &emptypb.Empty{})
	suite.Require().NoError(err)

	_, err = suite.Client.MachineClient.CPUInfo(nodeCtx, &emptypb.Empty{})
	suite.Require().NoError(err)

	_, err = suite.Client.MachineClient.DiskStats(nodeCtx, &emptypb.Empty{})
	suite.Require().NoError(err)

	_, err = suite.Client.MachineClient.LoadAvg(nodeCtx, &emptypb.Empty{})
	suite.Require().NoError(err)

	_, err = suite.Client.MachineClient.Memory(nodeCtx, &emptypb.Empty{})
	suite.Require().NoError(err)

	_, err = suite.Client.MachineClient.NetworkDeviceStats(nodeCtx, &emptypb.Empty{})
	suite.Require().NoError(err)

	_, err = suite.Client.MachineClient.SystemStat(nodeCtx, &emptypb.Empty{})
	suite.Require().NoError(err)
}

func init() {
	allSuites = append(allSuites, new(MonitoringSuite))
}
