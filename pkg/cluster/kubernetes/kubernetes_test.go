// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes_test

import (
	"fmt"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestUpgradePath(t *testing.T) {
	// ensure that upgrade path is available for n-5 supported Kubernetes versions
	latestVersion, err := semver.ParseTolerant(constants.DefaultKubernetesVersion)
	require.NoError(t, err)

	for minorVersion := latestVersion.Minor - constants.SupportedKubernetesVersions + 1; minorVersion <= latestVersion.Minor; minorVersion++ {
		thisVersion := fmt.Sprintf("%d.%d", latestVersion.Major, minorVersion)

		path, err := upgrade.NewPath(thisVersion, thisVersion)
		require.NoError(t, err)

		assert.True(t, path.IsSupported(), "upgrade path %s is not supported", path.String())

		if minorVersion != latestVersion.Minor {
			nextVersion := fmt.Sprintf("%d.%d", latestVersion.Major, minorVersion+1)

			path, err = upgrade.NewPath(thisVersion, nextVersion)
			require.NoError(t, err)

			assert.True(t, path.IsSupported(), "upgrade path %s is not supported", path.String())
		}
	}
}
