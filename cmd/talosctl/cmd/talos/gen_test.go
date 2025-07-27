// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestValidateTalosVersion(t *testing.T) {
	// Extract the current version components
	currentVersion := constants.Version
	
	// Validation should pass for empty version (defaults to current)
	err := validateTalosVersion("")
	assert.NoError(t, err)

	// Validation should pass for current version
	err = validateTalosVersion(currentVersion)
	assert.NoError(t, err)

	// Validation should pass for older versions
	err = validateTalosVersion("1.0.0")
	assert.NoError(t, err)

	// Extract major.minor from current version to test next minor
	var currentMajor, currentMinor int
	_, err = require.NoError(t, 
		fmt.Sscanf(currentVersion, "%d.%d", &currentMajor, &currentMinor))
	
	// Validation should pass for next minor version
	nextMinor := fmt.Sprintf("%d.%d.0", currentMajor, currentMinor+1)
	err = validateTalosVersion(nextMinor)
	assert.NoError(t, err)

	// Validation should fail for versions more than one minor ahead
	tooFarAhead := fmt.Sprintf("%d.%d.0", currentMajor, currentMinor+2)
	err = validateTalosVersion(tooFarAhead)
	assert.Error(t, err)
	
	// Validation should fail for versions with higher major version
	nextMajor := fmt.Sprintf("%d.0.0", currentMajor+1)
	err = validateTalosVersion(nextMajor)
	assert.Error(t, err)

	// Validation should fail for invalid version format
	err = validateTalosVersion("not-a-version")
	assert.Error(t, err)
}
