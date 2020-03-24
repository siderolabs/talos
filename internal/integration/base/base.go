// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package base provides shared definition of base suites for tests
package base

import (
	"github.com/talos-systems/talos/internal/pkg/provision"
)

// TalosSuite defines most common settings for integration test suites
type TalosSuite struct {
	// Endpoint to use to connect, if not set config is used
	Endpoint string
	// K8sEndpoint is API server endpoint, if set overrides kubeconfig
	K8sEndpoint string
	// Cluster describes provisioned cluster, used for discovery purposes
	Cluster provision.Cluster
	// TalosConfig is a path to talosconfig
	TalosConfig string
	// Version is the (expected) version of Talos tests are running against
	Version string
	// TalosctlPath is path to talosctl binary
	TalosctlPath string

	discoveredNodes []string
}

// DiscoverNodes provides basic functionality to discover cluster nodes via test settings.
//
// This method is overridden in specific suites to allow for specific discovery.
func (talosSuite *TalosSuite) DiscoverNodes() []string {
	if talosSuite.discoveredNodes == nil {
		if talosSuite.Cluster != nil {
			for _, node := range talosSuite.Cluster.Info().Nodes {
				talosSuite.discoveredNodes = append(talosSuite.discoveredNodes, node.PrivateIP.String())
			}
		}
	}

	return talosSuite.discoveredNodes
}

// ConfiguredSuite expects config to be set before running
type ConfiguredSuite interface {
	SetConfig(config TalosSuite)
}

// SetConfig implements ConfiguredSuite
func (suite *TalosSuite) SetConfig(config TalosSuite) {
	*suite = config
}

// NamedSuite interface provides names for test suites
type NamedSuite interface {
	SuiteName() string
}
