// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talos17 provides compatibility constants for Talos 1.7.
package talos17

import (
	"github.com/blang/semver/v4"
)

// MajorMinor is the major.minor version of Talos 1.7.
var MajorMinor = [2]uint64{1, 7}

// MinimumHostUpgradeVersion is the minimum version of Talos that can be upgraded to 1.7.
var MinimumHostUpgradeVersion = semver.MustParse("1.4.0")

// MaximumHostDowngradeVersion is the maximum (not inclusive) version of Talos that can be downgraded to 1.7.
var MaximumHostDowngradeVersion = semver.MustParse("1.9.0")

// DeniedHostUpgradeVersions are the versions of Talos that cannot be upgraded to 1.7.
var DeniedHostUpgradeVersions []semver.Version

// MinimumKubernetesVersion is the minimum version of Kubernetes is supported with 1.7.
var MinimumKubernetesVersion = semver.MustParse("1.25.0")

// MaximumKubernetesVersion is the maximum version of Kubernetes is supported with 1.7.
var MaximumKubernetesVersion = semver.MustParse("1.30.99")
