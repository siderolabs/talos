// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// EnvironmentSuite verifies Environment API.
type EnvironmentSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EnvironmentSuite) SuiteName() string {
	return "api.EnvironmentSuite"
}

// SetupTest ...
func (suite *EnvironmentSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *EnvironmentSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestEnvironment tests setting environment variables via Environment API.
func (suite *EnvironmentSuite) TestEnvironment() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	suite.Require().Eventually(func() bool {
		return suite.validateEnvironment(node, []string{"TALOS_TEST_ENV=1"}, false)
	}, 5*time.Second, 1*time.Second, "environment variable was there before apply")

	suite.T().Logf("applying environment configuration")

	doc := runtimecfg.NewEnvironmentV1Alpha1()
	doc.EnvironmentVariables = map[string]string{"TALOS_TEST_ENV": "1"}

	suite.PatchMachineConfig(ctx, doc)

	suite.Require().Eventually(func() bool {
		return suite.validateEnvironment(node, []string{"TALOS_TEST_ENV=1"}, true)
	}, 5*time.Second, 1*time.Second, "environment variable was not set after apply")

	// now we want to reboot the node and make sure the env is retained
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
		suite.CleanupFailedPods,
	)

	suite.Require().Eventually(func() bool {
		return suite.validateEnvironment(node, []string{"TALOS_TEST_ENV=1"}, true)
	}, 5*time.Second, 1*time.Second, "environment variable was not retained after reboot")

	suite.T().Logf("removing environment configuration")

	suite.RemoveMachineConfigDocuments(ctx, runtimecfg.EnvironmentConfigKind)

	// now we want to reboot the node and make sure the env is removed
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
		suite.CleanupFailedPods,
	)

	suite.Require().Eventually(func() bool {
		return suite.validateEnvironment(node, []string{"TALOS_TEST_ENV=1"}, false)
	}, 5*time.Second, 1*time.Second, "environment variable was not removed after reboot")
}

func (suite *EnvironmentSuite) validateEnvironment(node string, expectedVariables []string, shouldContain bool) bool {
	ctx := client.WithNode(suite.ctx, node)

	env, err := safe.StateGetByID[*runtime.Environment](ctx, suite.Client.COSI, "machined")
	suite.Require().NoError(err)

	for _, v := range expectedVariables {
		if slices.Contains(env.TypedSpec().Variables, v) != shouldContain {
			return false
		}
	}

	return true
}

func init() {
	allSuites = append(allSuites, new(EnvironmentSuite))
}
