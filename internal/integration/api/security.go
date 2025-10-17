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

// SecuritySuite verifies the security state resource.
type SecuritySuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName returns the name of the suite.
func (suite *SecuritySuite) SuiteName() string {
	return "api.SecuritySuite"
}

// SetupTest sets up the test.
func (suite *SecuritySuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)

	if suite.Cluster != nil && suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping Security test since provisioner is not docker")
	}
}

// TearDownTest tears down the test.
func (suite *SecuritySuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestSecurityState verifies that the security state resource is present and has valid values.
func (suite *SecuritySuite) TestSecurityState() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{runtimeres.SecurityStateID},
		func(r *runtimeres.SecurityState, asrt *assert.Assertions) {
			asrt.True(r.TypedSpec().ModuleSignatureEnforced, "module signature enforcement should be enabled")
		},
	)
}

func init() {
	allSuites = append(allSuites, &SecuritySuite{})
}
