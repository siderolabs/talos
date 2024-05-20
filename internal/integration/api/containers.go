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

// ContainersSuite ...
type ContainersSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ContainersSuite) SuiteName() string {
	return "api.ContainersSuite"
}

// SetupTest ...
func (suite *ContainersSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Minute)
}

// TearDownTest ...
func (suite *ContainersSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestSandboxImage verifies sandbox image.
func (suite *ContainersSuite) TestSandboxImage() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	ctx := client.WithNode(suite.ctx, node)

	resp, err := suite.Client.Containers(ctx, constants.K8sContainerdNamespace, common.ContainerDriver_CRI)
	suite.Require().NoError(err)

	suite.Assert().NotEmpty(resp.GetMessages())

	for _, message := range resp.GetMessages() {
		suite.Assert().NotEmpty(message.GetContainers())

		for _, ctr := range message.GetContainers() {
			if ctr.PodId == "" {
				suite.Assert().Equal(images.DefaultSandboxImage, ctr.Image)
			}
		}
	}
}

func init() {
	allSuites = append(allSuites, new(ContainersSuite))
}
