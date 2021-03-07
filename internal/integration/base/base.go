// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package base provides shared definition of base suites for tests
package base

import (
	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/access"
)

// TalosSuite defines most common settings for integration test suites.
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
	// TalosctlPath is a path to talosctl binary
	TalosctlPath string
	// KubectlPath is a path to kubectl binary
	KubectlPath string

	discoveredNodes cluster.Info
}

// DiscoverNodes provides basic functionality to discover cluster nodes via test settings.
//
// This method is overridden in specific suites to allow for specific discovery.
func (talosSuite *TalosSuite) DiscoverNodes() cluster.Info {
	if talosSuite.discoveredNodes == nil {
		if talosSuite.Cluster != nil {
			talosSuite.discoveredNodes = access.NewAdapter(talosSuite.Cluster).Info
		}
	}

	return talosSuite.discoveredNodes
}

// ConfiguredSuite expects config to be set before running.
type ConfiguredSuite interface {
	SetConfig(config TalosSuite)
}

// SetConfig implements ConfiguredSuite.
func (talosSuite *TalosSuite) SetConfig(config TalosSuite) {
	*talosSuite = config
}

// NamedSuite interface provides names for test suites.
type NamedSuite interface {
	SuiteName() string
}
