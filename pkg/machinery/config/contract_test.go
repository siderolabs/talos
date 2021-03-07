// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

func TestContractGreater(t *testing.T) {
	assert.True(t, config.TalosVersion0_9.Greater(config.TalosVersion0_8))
	assert.True(t, config.TalosVersionCurrent.Greater(config.TalosVersion0_8))
	assert.True(t, config.TalosVersionCurrent.Greater(config.TalosVersion0_9))

	assert.False(t, config.TalosVersion0_8.Greater(config.TalosVersion0_9))
	assert.False(t, config.TalosVersion0_8.Greater(config.TalosVersion0_8))
	assert.False(t, config.TalosVersionCurrent.Greater(config.TalosVersionCurrent))
}

func TestContractParseVersion(t *testing.T) {
	contract, err := config.ParseContractFromVersion("v0.8")
	assert.NoError(t, err)
	assert.Equal(t, config.TalosVersion0_8, contract)

	contract, err = config.ParseContractFromVersion("v0.8.1")
	assert.NoError(t, err)
	assert.Equal(t, config.TalosVersion0_8, contract)

	contract, err = config.ParseContractFromVersion("v0.88")
	assert.NoError(t, err)
	assert.NotEqual(t, config.TalosVersion0_8, contract)

	contract, err = config.ParseContractFromVersion("v0.8.3-alpha.4")
	assert.NoError(t, err)
	assert.Equal(t, config.TalosVersion0_8, contract)
}

func TestContractCurrent(t *testing.T) {
	assert.True(t, config.TalosVersionCurrent.SupportsAggregatorCA())
	assert.True(t, config.TalosVersionCurrent.SupportsECDSAKeys())
	assert.True(t, config.TalosVersionCurrent.SupportsServiceAccount())
}

func TestContract0_9(t *testing.T) {
	assert.True(t, config.TalosVersion0_9.SupportsAggregatorCA())
	assert.True(t, config.TalosVersion0_9.SupportsECDSAKeys())
	assert.True(t, config.TalosVersion0_9.SupportsServiceAccount())
}

func TestContract0_8(t *testing.T) {
	assert.False(t, config.TalosVersion0_8.SupportsAggregatorCA())
	assert.False(t, config.TalosVersion0_8.SupportsECDSAKeys())
	assert.False(t, config.TalosVersion0_8.SupportsServiceAccount())
}
