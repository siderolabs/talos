// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package provision provides integration tests which rely on provisioning cluster per test.
package provision

import (
	"fmt"
	"regexp"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/version"
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
	DiskGB uint64
	// Node count for the tests
	MasterNodes int
	WorkerNodes int
	// Target installer image registry
	TargetInstallImageRegistry string
	// Current version of the cluster (built in the CI pass)
	CurrentVersion string
	// Custom CNI URL to use.
	CustomCNIURL string
	// Enable crashdump on failure.
	CrashdumpEnabled bool
	// CNI bundle for QEMU provisioner.
	CNIBundleURL string
}

// DefaultSettings filled in by test runner.
var DefaultSettings = Settings{
	CIDR:                       "172.21.0.0/24",
	MTU:                        1500,
	CPUs:                       2,
	MemMB:                      2 * 1024,
	DiskGB:                     8,
	MasterNodes:                3,
	WorkerNodes:                1,
	TargetInstallImageRegistry: "ghcr.io",
	CNIBundleURL:               fmt.Sprintf("https://github.com/talos-systems/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz", trimVersion(version.Tag), constants.ArchVariable),
}

func trimVersion(version string) string {
	// remove anything extra after semantic version core, `v0.3.2-1-abcd` -> `v0.3.2`
	return regexp.MustCompile(`(-\d+-g[0-9a-f]+)$`).ReplaceAllString(version, "")
}
