// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubelet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/kubelet"
)

func TestValidateCPUReservationConfiguration(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		extraConfig map[string]any
		extraArgs   map[string][]string

		expectedError string
	}{
		{
			name: "no settings",
		},
		{
			name: "unrelated settings",
			extraConfig: map[string]any{
				"cpuManagerPolicy": "static",
				"systemReserved":   map[string]any{"cpu": "500m"},
			},
			extraArgs: map[string][]string{"cpu-manager-policy": {"static"}},
		},
		{
			name:          "configuration field",
			extraConfig:   map[string]any{"reservedSystemCPUs": "0-1"},
			expectedError: `"reservedSystemCPUs" is derived from the CPU partition kubepods root`,
		},
		{
			name:          "flag",
			extraArgs:     map[string][]string{"reserved_cpus": {"0-1"}},
			expectedError: `"reserved-cpus" is derived from the CPU partition kubepods root`,
		},
		{
			name:          "other CPU manager policy",
			extraConfig:   map[string]any{"cpuManagerPolicy": "none"},
			expectedError: `"cpuManagerPolicy" none conflicts with the CPU partition kubepods root`,
		},
		{
			name:          "other CPU manager policy flag",
			extraArgs:     map[string][]string{"cpu_manager_policy": {"none"}},
			expectedError: `"cpu-manager-policy" conflicts with the CPU partition kubepods root`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := kubelet.ValidateCPUReservationConfiguration(test.extraConfig, test.extraArgs)

			if test.expectedError == "" {
				assert.NoError(t, err)

				return
			}

			assert.ErrorContains(t, err, test.expectedError)
		})
	}
}
