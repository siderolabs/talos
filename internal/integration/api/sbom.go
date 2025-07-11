// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// SBOMSuite ...
type SBOMSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *SBOMSuite) SuiteName() string {
	return "api.SBOMSuite"
}

// SetupTest ...
func (suite *SBOMSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)
}

// TearDownTest ...
func (suite *SBOMSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestCommon verifies that common SBOM items are available.
func (suite *SBOMSuite) TestCommon() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI,
		[]resource.ID{
			// list of common SBOM items which should be present always
			"Talos",
			"github.com/siderolabs/go-kubernetes",
		},
		func(item *runtime.SBOMItem, asrt *assert.Assertions) {
			asrt.NotEmpty(item.TypedSpec().Name, "SBOM item name should not be empty")
			asrt.NotEmpty(item.TypedSpec().Version, "SBOM item version should not be empty")
		},
	)

	// Talos SBOM item should have a matching version.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI,
		"Talos",
		func(item *runtime.SBOMItem, asrt *assert.Assertions) {
			asrt.Equal(version.Name, item.TypedSpec().Name, "SBOM item name should match Talos version name")
			asrt.Equal(version.Tag, item.TypedSpec().Version, "SBOM item version should match Talos version")
		},
	)

	// Assert on containerd/runc versions.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI,
		"containerd",
		func(item *runtime.SBOMItem, asrt *assert.Assertions) {
			asrt.Equal("v"+constants.DefaultContainerdVersion, item.TypedSpec().Version)
		},
	)
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI,
		"runc",
		func(item *runtime.SBOMItem, asrt *assert.Assertions) {
			asrt.Equal("v"+constants.RuncVersion, item.TypedSpec().Version)
		},
	)

	// Assert on Go version.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI,
		"golang",
		func(item *runtime.SBOMItem, asrt *assert.Assertions) {
			goVersion := strings.TrimPrefix(constants.GoVersion, "go")

			asrt.Equal(goVersion, item.TypedSpec().Version)
		},
	)

	if suite.Capabilities().RunsTalosKernel {
		// Assert on Talos kernel version.
		rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI,
			"kernel",
			func(item *runtime.SBOMItem, asrt *assert.Assertions) {
				// cut the suffix
				version, _, ok := strings.Cut(constants.DefaultKernelVersion, "-")
				suite.Require().True(ok, "kernel version should have a suffix")

				asrt.Equal(version, item.TypedSpec().Version)
			},
		)
	} else {
		rtestutils.AssertNoResource[*runtime.SBOMItem](ctx, suite.T(), suite.Client.COSI, "kernel")
	}
}

func init() {
	allSuites = append(allSuites, new(SBOMSuite))
}
