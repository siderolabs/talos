// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// SandboxImageSuite verifies the image the CRI runs pod sandboxes with.
//
// Kept apart from ContainersSuite: this only lists containers which are already there, so unlike the
// tests for declared containers it neither pulls an image nor needs a long deadline.
type SandboxImageSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *SandboxImageSuite) SuiteName() string {
	return "api.SandboxImageSuite"
}

// SetupTest ...
func (suite *SandboxImageSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Minute)
}

// TearDownTest ...
func (suite *SandboxImageSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestSandboxImage verifies sandbox image.
func (suite *SandboxImageSuite) TestSandboxImage() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	ctx := client.WithNode(suite.ctx, node)

	resp, err := suite.Client.Containers(ctx, constants.K8sContainerdNamespace, common.ContainerDriver_CRI)
	suite.Require().NoError(err)

	suite.Assert().NotEmpty(resp.GetMessages())

	for _, message := range resp.GetMessages() {
		suite.Assert().NotEmpty(message.GetContainers())

		matched := false

		for _, ctr := range message.GetContainers() {
			if ctr.PodId == ctr.Id {
				suite.Assert().Equal(images.DefaultSandboxImage, ctr.Image)

				matched = true
			}
		}

		suite.Assert().True(matched, "no pods found, node %s", node)
	}
}

func init() {
	allSuites = append(allSuites, new(SandboxImageSuite))
}
