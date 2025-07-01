// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// KernelSuite ...
type KernelSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *KernelSuite) SuiteName() string {
	return "api.KernelSuite"
}

// SetupTest ...
func (suite *KernelSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Second)

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("skipping kernel test since Talos kernel is not running")
	}
}

// TearDownTest ...
func (suite *KernelSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestCmdline tests the /proc/cmdline resource.
func (suite *KernelSuite) TestCmdline() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{runtime.KernelCmdlineID},
		func(res *runtime.KernelCmdline, asrt *assert.Assertions) {
			asrt.NotEmpty(res.TypedSpec().Cmdline, "kernel cmdline should not be empty")
		},
	)
}

func init() {
	allSuites = append(allSuites, new(KernelSuite))
}
