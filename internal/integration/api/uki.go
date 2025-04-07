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
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// UKISuite verifies Talos is booted from a UKI.
type UKISuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName returns the name of the suite.
func (suite *UKISuite) SuiteName() string {
	return "api.UKISuite"
}

// SetupTest sets up the test.
func (suite *UKISuite) SetupTest() {
	if suite.Cluster != nil && suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping uki booted test since docker provisioner does not support UKI")
	}

	if !suite.VerifyUKIBooted {
		suite.T().Skip("skipping uki booted test since talos.verifyukibooted is false")
	}

	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)
}

// TearDownTest tears down the test.
func (suite *UKISuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestUKIBooted verifies that the system is booted from a UKI.
func (suite *UKISuite) TestUKIBooted() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{runtimeres.SecurityStateID},
		func(r *runtimeres.SecurityState, asrt *assert.Assertions) {
			asrt.True(r.TypedSpec().BootedWithUKI)
		},
	)
}

func init() {
	allSuites = append(allSuites, &UKISuite{})
}
