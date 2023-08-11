// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// TrustedBootSuite verifies Talos is securebooted.
type TrustedBootSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *TrustedBootSuite) SuiteName() string {
	return "api.TrustedBootSuite"
}

// SetupTest ...
func (suite *TrustedBootSuite) SetupTest() {
	if suite.Cluster.Provisioner() == provisionerDocker {
		suite.T().Skip("skipping trustedboot tests in docker")
	}

	if !suite.TrustedBoot {
		suite.T().Skip("skipping since talos.trustedboot is false")
	}

	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)
}

// TearDownTest ...
func (suite *TrustedBootSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestTrustedBootState verifies that the system is booted in secure boot mode
// and that the disks are encrypted.
func (suite *TrustedBootSuite) TestTrustedBootState() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	securityResource, err := safe.StateWatchFor[*runtimeres.SecurityState](
		ctx,
		suite.Client.COSI,
		runtimeres.NewSecurityStateSpec(runtimeres.NamespaceName).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	suite.Require().NoError(err)

	suite.Require().True(securityResource.TypedSpec().SecureBoot)

	stateResource, err := safe.StateWatchFor[*runtimeres.MountStatus](
		ctx,
		suite.Client.COSI,
		runtimeres.NewMountStatus(runtimeres.NamespaceName, resource.ID("STATE")).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	suite.Require().NoError(err)

	suite.Require().True(stateResource.TypedSpec().Encrypted)

	ephemeralResource, err := safe.StateWatchFor[*runtimeres.MountStatus](
		ctx,
		suite.Client.COSI,
		runtimeres.NewMountStatus(runtimeres.NamespaceName, resource.ID("EPHEMERAL")).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	suite.Require().NoError(err)

	suite.Require().True(ephemeralResource.TypedSpec().Encrypted)

	dmesgStream, err := suite.Client.Dmesg(
		suite.ctx,
		false,
		false,
	)
	suite.Require().NoError(err)

	logReader, err := client.ReadStream(dmesgStream)
	suite.Require().NoError(err)

	var dmesg bytes.Buffer
	_, err = io.Copy(bufio.NewWriter(&dmesg), logReader)
	suite.Require().NoError(err)

	suite.Require().Contains(dmesg.String(), "Secure boot enabled")
}

func init() {
	allSuites = append(allSuites, &TrustedBootSuite{})
}
