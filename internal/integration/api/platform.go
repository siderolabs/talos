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
	"gopkg.in/yaml.v3"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// PlatformSuite ...
type PlatformSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *PlatformSuite) SuiteName() string {
	return "api.PlatformSuite"
}

// SetupTest ...
func (suite *PlatformSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)
}

// TearDownTest ...
func (suite *PlatformSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestPlatformMetadata verifies platform metadata.
func (suite *PlatformSuite) TestPlatformMetadata() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, runtime.PlatformMetadataID, func(md *runtime.PlatformMetadata, asrt *assert.Assertions) {
		marshaled, err := resource.MarshalYAML(md)
		suite.Require().NoError(err)

		yml, err := yaml.Marshal(marshaled)
		suite.Require().NoError(err)

		suite.T().Logf("platform metadata:\n%s", string(yml))

		if md.TypedSpec().Platform == "aws" || md.TypedSpec().Platform == "gcp" {
			asrt.NotEmpty(md.TypedSpec().Tags)
		}
	})
}

// TestMetalPlatformMetadata verifies platform metadata for metal platform.
func (suite *PlatformSuite) TestMetalPlatformMetadata() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping platform metal test since provisioner is not qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("verifying metal platform network config on node %s", node)

	const linkName = "dummy-platform"

	platformNetworkConfig := v1alpha1runtime.PlatformNetworkConfig{
		Links: []network.LinkSpecSpec{
			{
				Name:    linkName,
				Logical: true,
				Up:      false,
				MTU:     1500,
				Type:    nethelpers.LinkEther,
				Kind:    "dummy",
			},
		},
	}

	platformNetworkConfigMarshaled, err := yaml.Marshal(platformNetworkConfig)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Client.MetaWrite(ctx, meta.MetalNetworkPlatformConfig, platformNetworkConfigMarshaled))

	// link should appear on the node
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, linkName, func(*network.LinkStatus, *assert.Assertions) {})

	suite.Require().NoError(suite.Client.MetaDelete(ctx, meta.MetalNetworkPlatformConfig))

	// address should be removed
	rtestutils.AssertNoResource[*network.LinkStatus](ctx, suite.T(), suite.Client.COSI, linkName)
}

func init() {
	allSuites = append(allSuites, new(PlatformSuite))
}
