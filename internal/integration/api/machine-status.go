// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api
// +build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/resources/runtime"
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
	ctx, cancel := context.WithTimeout(client.WithNodes(suite.ctx, node), 30*time.Second)
	defer cancel()

	watchClient, err := suite.Client.Resources.Watch(
		ctx,
		runtime.NamespaceName,
		runtime.MachineStatusType,
		runtime.MachineStatusID)
	if err != nil {
		return err
	}

	for {
		msg, err := watchClient.Recv()
		if err != nil {
			if client.StatusCode(err) == codes.DeadlineExceeded {
				return fmt.Errorf("%s: timed out waiting for MachineStatus to be ready", node)
			}

			return fmt.Errorf("%s: %w", node, err)
		}

		if msg.Metadata.GetError() != "" {
			return fmt.Errorf("%s: %s", msg.Metadata.GetHostname(), msg.Metadata.GetError())
		}

		if msg.Resource == nil {
			continue
		}

		b, err := yaml.Marshal(msg.Resource.Spec())
		if err != nil {
			return err
		}

		var spec runtime.MachineStatusSpec

		if err = yaml.Unmarshal(b, &spec); err != nil {
			return err
		}

		if spec.Stage == runtime.MachineStageRunning && spec.Status.Ready {
			return nil
		}

		suite.T().Logf("%s: MachineStatus stage %s ready %v, unmetConditions %v", node, spec.Stage, spec.Status.Ready, spec.Status.UnmetConditions)
	}
}

func init() {
	allSuites = append(allSuites, new(MachineStatusSuite))
}
