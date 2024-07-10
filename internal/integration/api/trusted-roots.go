// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TrustedRootsSuite ...
type TrustedRootsSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *TrustedRootsSuite) SuiteName() string {
	return "api.TrustedRootsSuite"
}

// SetupTest ...
func (suite *TrustedRootsSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)
}

// TearDownTest ...
func (suite *TrustedRootsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *TrustedRootsSuite) readTrustedRoots(nodeCtx context.Context) string {
	r, err := suite.Client.Read(nodeCtx, constants.DefaultTrustedCAFile)
	suite.Require().NoError(err)

	value, err := io.ReadAll(r)
	suite.Require().NoError(err)

	suite.Require().NoError(r.Close())

	return string(value)
}

// TestTrustedRoots verifies default and custom trusted CA roots.
func (suite *TrustedRootsSuite) TestTrustedRoots() {
	// pick up a random node to test the TrustedRoots on, and use it throughout the test
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Logf("testing TrustedRoots on node %s", node)

	// build a Talos API context which is tied to the node
	nodeCtx := client.WithNode(suite.ctx, node)

	const name = "test-ca"

	cfgDocument := security.NewTrustedRootsConfigV1Alpha1()
	cfgDocument.MetaName = name
	cfgDocument.Certificates = "--- BEGIN CERTIFICATE ---\nMIIC0DCCAbigAwIBAgIUI\n--- END CERTIFICATE ---\n"

	// clean up custom config if it exists
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	certificates := suite.readTrustedRoots(nodeCtx)
	suite.Require().Contains(certificates, "Bundle of CA Root Certificates")

	// enable custom trusted roots
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	suite.Require().Eventually(func() bool {
		return strings.Contains(suite.readTrustedRoots(nodeCtx), name)
	}, 5*time.Second, 100*time.Millisecond)

	// deactivate the TrustedRoots
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	suite.Require().Eventually(func() bool {
		return !strings.Contains(suite.readTrustedRoots(nodeCtx), name)
	}, 5*time.Second, 100*time.Millisecond)
}

func init() {
	allSuites = append(allSuites, new(TrustedRootsSuite))
}
