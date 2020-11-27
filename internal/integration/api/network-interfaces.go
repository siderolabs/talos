// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"strings"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
)

// NetworkInterfacesSuite ...
type NetworkInterfacesSuite struct {
	base.K8sSuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *NetworkInterfacesSuite) SuiteName() string {
	return "api.NetworkInterfacesSuite"
}

// SetupTest ...
func (suite *NetworkInterfacesSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for Recovers
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *NetworkInterfacesSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestGenerate verifies the generate config API.
func (suite *NetworkInterfacesSuite) TestGenerate() {
	reply, err := suite.Client.Interfaces(
		suite.ctx,
	)

	suite.Require().NoError(err)

	suite.Require().Greater(len(reply.Messages[0].Interfaces), 0)

	found := false
	// try to find lo
	for _, adapter := range reply.Messages[0].Interfaces {
		if strings.HasPrefix(adapter.Name, "lo") {
			found = true

			break
		}
	}

	suite.Require().True(found)
}

func init() {
	allSuites = append(allSuites, new(NetworkInterfacesSuite))
}
