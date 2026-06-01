// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestParseDynamicModuleState(t *testing.T) {
	for _, tt := range []struct {
		input    string
		expected runtime.KernelModuleState
		wantErr  bool
	}{
		{"Live", runtime.KernelModuleStateLive, false},
		{"Loading", runtime.KernelModuleStateLoading, false},
		{"Unloading", runtime.KernelModuleStateUnloading, false},
		{"", 0, true},
		{"unknown", 0, true},
	} {
		t.Run(tt.input, func(t *testing.T) {
			result, err := runtime.ParseDynamicModuleState(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
