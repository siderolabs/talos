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

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
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

		if md.TypedSpec().Platform == "aws" {
			asrt.NotEmpty(md.TypedSpec().Tags)
		}
	})
}

func init() {
	allSuites = append(allSuites, new(PlatformSuite))
}
