// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package base

import (
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/tls"
)

// APISuite is a base suite for API tests
type APISuite struct {
	suite.Suite
	TalosSuite

	Client *client.Client
}

// SetupSuite initializes Talos API client
func (apiSuite *APISuite) SetupSuite() {
	configContext, creds, err := client.NewClientContextAndCredentialsFromConfig(apiSuite.TalosConfig, "")
	apiSuite.Require().NoError(err)

	endpoints := configContext.Endpoints
	if apiSuite.Endpoint != "" {
		endpoints = []string{apiSuite.Endpoint}
	}

	tlsconfig, err := tls.New(
		tls.WithKeypair(creds.Crt),
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(creds.CA),
	)
	if err != nil {
		apiSuite.Require().NoError(err)
	}

	apiSuite.Client, err = client.NewClient(tlsconfig, endpoints, constants.ApidPort)
	apiSuite.Require().NoError(err)
}

// DiscoverNodes provides list of Talos nodes in the cluster.
//
// As there's no way to provide this functionality via Talos API, it works the following way:
// 1. If there's a provided list of nodes, it's used.
// 2. If integration test was compiled with k8s support, k8s is used.
func (apiSuite *APISuite) DiscoverNodes() []string {
	discoveredNodes := apiSuite.TalosSuite.DiscoverNodes()
	if discoveredNodes != nil {
		return discoveredNodes
	}

	var err error

	apiSuite.discoveredNodes, err = discoverNodesK8s(apiSuite.Client, &apiSuite.TalosSuite)
	apiSuite.Require().NoError(err, "k8s discovery failed")

	if apiSuite.discoveredNodes == nil {
		// still no nodes, skip the test
		apiSuite.T().Skip("no nodes were discovered")
	}

	return apiSuite.discoveredNodes
}

// TearDownSuite closes Talos API client
func (apiSuite *APISuite) TearDownSuite() {
	if apiSuite.Client != nil {
		apiSuite.Assert().NoError(apiSuite.Client.Close())
	}
}
