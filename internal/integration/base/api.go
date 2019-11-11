// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package base

import (
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/pkg/constants"
)

// APISuite is a base suite for API tests
type APISuite struct {
	suite.Suite
	TalosSuite

	Client *client.Client
}

// SetupSuite initializes Talos API client
func (apiSuite *APISuite) SetupSuite() {
	target, creds, err := client.NewClientTargetAndCredentialsFromConfig(apiSuite.TalosConfig, "")
	apiSuite.Require().NoError(err)

	if apiSuite.Target != "" {
		target = apiSuite.Target
	}

	apiSuite.Client, err = client.NewClient(creds, target, constants.OsdPort)
	apiSuite.Require().NoError(err)
}

// TearDownSuite closes Talos API client
func (apiSuite *APISuite) TearDownSuite() {
	if apiSuite.Client != nil {
		apiSuite.Assert().NoError(apiSuite.Client.Close())
	}
}
