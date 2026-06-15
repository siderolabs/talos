// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

//go:embed testdata/securityprofileconfig.yaml
var expectedSecurityProfileConfigDocument []byte

func TestSecurityProfileConfigMarshalStability(t *testing.T) {
	cfg := runtime.NewSecurityProfileConfigV1Alpha1()
	cfg.WorkloadIsolationEnabled = new(true)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedSecurityProfileConfigDocument, marshaled)
}

func TestSecurityProfileConfigWorkloadIsolation(t *testing.T) {
	t.Parallel()

	// absent field -> disabled
	assert.False(t, runtime.NewSecurityProfileConfigV1Alpha1().WorkloadIsolation())

	cfg := runtime.NewSecurityProfileConfigV1Alpha1()
	cfg.WorkloadIsolationEnabled = new(true)
	assert.True(t, cfg.WorkloadIsolation())

	cfg.WorkloadIsolationEnabled = new(false)
	assert.False(t, cfg.WorkloadIsolation())

	warnings, err := cfg.Validate(validationMode{})
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
