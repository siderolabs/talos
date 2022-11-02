// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MachineStatusSuite ...
type MachineStatusSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *MachineStatusSuite) SuiteName() string {
	return "api.MachineStatusSuite"
}

// SetupTest ...
func (suite *MachineStatusSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for MachineStatuss
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *MachineStatusSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestMachineStatusReady tests that MachineStatus is eventually ready & running.
func (suite *MachineStatusSuite) TestMachineStatusReady() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	for _, node := range nodes {
		suite.Assert().NoError(suite.waitMachineStatusReady(node))
	}
}

//nolint:gocyclo
func (suite *MachineStatusSuite) waitMachineStatusReady(node string) error {
	ctx, cancel := context.WithTimeout(client.WithNode(suite.ctx, node), 30*time.Second)
	defer cancel()

	watchCh := make(chan safe.WrappedStateEvent[*runtime.MachineStatus])

	if err := safe.StateWatch(
		ctx,
		suite.Client.COSI,
		resource.NewMetadata(runtime.NamespaceName, runtime.MachineStatusType, runtime.MachineStatusID, resource.VersionUndefined),
		watchCh,
	); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: timed out waiting for MachineStatus to be ready", node)
		case event := <-watchCh:
			machineStatus, err := event.Resource()
			if err != nil {
				return err
			}

			if machineStatus.TypedSpec().Stage == runtime.MachineStageRunning && machineStatus.TypedSpec().Status.Ready {
				return nil
			}

			suite.T().Logf(
				"%s: MachineStatus stage %s ready %v, unmetConditions %v",
				node, machineStatus.TypedSpec().Stage,
				machineStatus.TypedSpec().Status.Ready,
				machineStatus.TypedSpec().Status.UnmetConditions,
			)
		}
	}
}

func init() {
	allSuites = append(allSuites, new(MachineStatusSuite))
}
