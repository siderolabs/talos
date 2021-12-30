// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

func TestParseType(t *testing.T) {
	t.Parallel()

	t.Run("Values", func(t *testing.T) {
		// We have to use the same values as defined in proto as we use direct type conversions in many places.

		assert.EqualValues(t, machineapi.MachineConfig_TYPE_UNKNOWN, machine.TypeUnknown)
		assert.EqualValues(t, machineapi.MachineConfig_TYPE_INIT, machine.TypeInit)
		assert.EqualValues(t, machineapi.MachineConfig_TYPE_CONTROL_PLANE, machine.TypeControlPlane)
		assert.EqualValues(t, machineapi.MachineConfig_TYPE_WORKER, machine.TypeWorker)
	})

	validTests := []struct {
		s   string
		typ machine.Type
	}{
		{"init", machine.TypeInit},
		{"controlplane", machine.TypeControlPlane},
		{"worker", machine.TypeWorker},
		{"join", machine.TypeWorker},
		{"", machine.TypeWorker},
		{"unknown", machine.TypeUnknown},
	}

	for _, tt := range validTests {
		tt := tt
		t.Run(tt.s, func(t *testing.T) {
			t.Parallel()

			actual, err := machine.ParseType(tt.s)
			require.NoError(t, err)
			assert.Equal(t, tt.typ, actual)

			// check that constant's comment for stringer and ParseType() function are in sync
			actual, err = machine.ParseType(actual.String())
			require.NoError(t, err)
			assert.Equal(t, tt.typ, actual)
		})
	}

	for _, s := range []string{
		"foo",
	} {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Parallel()

			actual, err := machine.ParseType(s)
			assert.Error(t, err)
			assert.Equal(t, machine.TypeUnknown, actual)
		})
	}

	assert.NotEmpty(t, machine.TypeUnknown.String())
}
