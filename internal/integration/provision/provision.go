// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package provision provides integration tests which rely on on provisioning cluster per test.
package provision

import (
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/integration/base"
)

var allSuites []suite.TestingSuite

// GetAllSuites returns all the suites for provision test.
//
// Depending on build tags, this might return different lists.
func GetAllSuites() []suite.TestingSuite {
	return allSuites
}

// Settings for provision tests.
type Settings struct {
	// CIDR to use for provisioned clusters
	CIDR string
	// Registry mirrors to push to Talos config, in format `host=endpoint`
	RegistryMirrors base.StringList
	// MTU for the network.
	MTU int
	// VM parameters
	CPUs   int64
	MemMB  int64
	DiskGB int64
	// Node count for the tests
	MasterNodes int
	WorkerNodes int
	// Target installer image registry
	TargetInstallImageRegistry string
	// Current version of the cluster (built in the CI pass)
	CurrentVersion string
	// Custom CNI URL to use.
	CustomCNIURL string
}

// DefaultSettings filled in by test runner.
var DefaultSettings Settings = Settings{
	CIDR:                       "172.21.0.0/24",
	MTU:                        1500,
	CPUs:                       1,
	MemMB:                      1.5 * 1024,
	DiskGB:                     8,
	MasterNodes:                3,
	WorkerNodes:                1,
	TargetInstallImageRegistry: "docker.io",
}
