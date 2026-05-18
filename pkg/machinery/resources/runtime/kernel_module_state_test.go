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
	}{
		{"Live", runtime.KernelModuleStateActive},
		{"Loading", runtime.KernelModuleStateLoading},
		{"Unloading", runtime.KernelModuleStateUnloading},
		{"", runtime.KernelModuleStateInactive},
		{"unknown", runtime.KernelModuleStateInactive},
	} {
		t.Run(tt.input, func(t *testing.T) {
			result := runtime.ParseDynamicModuleState(tt.input)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected, result)
		})
	}
}
