// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeletstate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/kubeletstate"
)

func TestCPUManagerConfigFromKubelet(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg kubeletconfig.KubeletConfiguration

		expected      kubeletstate.CPUManagerConfig
		expectedError string
	}{
		{
			name: "defaults",
			expected: kubeletstate.CPUManagerConfig{
				Policy:       kubeletstate.CPUManagerPolicyNone,
				ReservedCPUs: cpuset.New(),
			},
		},
		{
			name: "reserved by quantity",
			cfg: kubeletconfig.KubeletConfiguration{
				CPUManagerPolicy: kubeletstate.CPUManagerPolicyStatic,
				KubeReserved:     map[string]string{"cpu": "100m", "memory": "1Gi"},
				SystemReserved:   map[string]string{"cpu": "1500m"},
			},
			expected: kubeletstate.CPUManagerConfig{
				Policy:          kubeletstate.CPUManagerPolicyStatic,
				ReservedCPUs:    cpuset.New(),
				NumReservedCPUs: 2,
			},
		},
		{
			name: "reserved explicitly",
			cfg: kubeletconfig.KubeletConfiguration{
				CPUManagerPolicy:        kubeletstate.CPUManagerPolicyStatic,
				CPUManagerPolicyOptions: map[string]string{"strict-cpu-reservation": "true", "full-pcpus-only": "true"},
				ReservedSystemCPUs:      "0,2,16,18",
				KubeReserved:            map[string]string{"cpu": "100m"},
			},
			expected: kubeletstate.CPUManagerConfig{
				Policy:               kubeletstate.CPUManagerPolicyStatic,
				StrictCPUReservation: true,
				ReservedCPUs:         cpuset.New(0, 2, 16, 18),
				NumReservedCPUs:      4,
			},
		},
		{
			name: "invalid reserved CPUs",
			cfg: kubeletconfig.KubeletConfiguration{
				ReservedSystemCPUs: "0-",
			},
			expectedError: `failed to parse reservedSystemCPUs "0-"`,
		},
		{
			name: "invalid policy option",
			cfg: kubeletconfig.KubeletConfiguration{
				CPUManagerPolicyOptions: map[string]string{"strict-cpu-reservation": "yes"},
			},
			expectedError: `failed to parse strict-cpu-reservation policy option "yes"`,
		},
		{
			name: "invalid quantity",
			cfg: kubeletconfig.KubeletConfiguration{
				SystemReserved: map[string]string{"cpu": "two"},
			},
			expectedError: `failed to parse reserved CPU quantity "two"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := kubeletstate.CPUManagerConfigFromKubelet(&test.cfg)

			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)

			assert.Equal(t, test.expected.Policy, actual.Policy)
			assert.Equal(t, test.expected.StrictCPUReservation, actual.StrictCPUReservation)
			assert.Equal(t, test.expected.NumReservedCPUs, actual.NumReservedCPUs)
			assert.True(t, test.expected.ReservedCPUs.Equals(actual.ReservedCPUs), "expected %s, got %s", test.expected.ReservedCPUs, actual.ReservedCPUs)
		})
	}
}

//nolint:maintidx
func TestValidateCPUManagerState(t *testing.T) {
	t.Parallel()

	// 8 CPUs, 0-1 reserved
	machine := kubeletstate.Machine{
		OnlineCPUs: cpuset.New(0, 1, 2, 3, 4, 5, 6, 7),
	}

	staticConfig := func(reserved cpuset.CPUSet, strict bool) kubeletstate.CPUManagerConfig {
		return kubeletstate.CPUManagerConfig{
			Policy:               kubeletstate.CPUManagerPolicyStatic,
			StrictCPUReservation: strict,
			ReservedCPUs:         reserved,
			NumReservedCPUs:      reserved.Size(),
		}
	}

	byQuantityConfig := func(num int, strict bool) kubeletstate.CPUManagerConfig {
		return kubeletstate.CPUManagerConfig{
			Policy:               kubeletstate.CPUManagerPolicyStatic,
			StrictCPUReservation: strict,
			ReservedCPUs:         cpuset.New(),
			NumReservedCPUs:      num,
		}
	}

	noneConfig := kubeletstate.CPUManagerConfig{
		Policy:       kubeletstate.CPUManagerPolicyNone,
		ReservedCPUs: cpuset.New(),
	}

	for _, test := range []struct {
		name string

		state   string
		cfg     kubeletstate.CPUManagerConfig
		machine kubeletstate.Machine

		expectedError string
	}{
		{
			name:  "none policy",
			state: `{"policyName":"none","defaultCpuSet":"","checksum":1353318690}`,
			cfg:   noneConfig,
		},
		{
			name:          "none to static",
			state:         `{"policyName":"none","defaultCpuSet":"","checksum":1353318690}`,
			cfg:           staticConfig(cpuset.New(0, 1), false),
			expectedError: `policy changed from "none" to "static"`,
		},
		{
			name:          "static to none",
			state:         `{"policyName":"static","defaultCpuSet":"0-7","checksum":1}`,
			cfg:           noneConfig,
			expectedError: `policy changed from "static" to "none"`,
		},
		{
			name:          "garbage",
			state:         `{"policyName":`,
			cfg:           noneConfig,
			expectedError: "failed to parse the state",
		},
		{
			name:  "static, empty state",
			state: `{"policyName":"static","defaultCpuSet":"","checksum":1}`,
			cfg:   staticConfig(cpuset.New(0, 1), false),
		},
		{
			name:  "static, empty state, strict",
			state: `{"policyName":"static","defaultCpuSet":"","checksum":1}`,
			cfg:   staticConfig(cpuset.New(0, 1), true),
		},
		{
			name:          "static, empty state with assignments",
			state:         `{"policyName":"static","defaultCpuSet":"","entries":{"pod1":{"c1":"2-3"}},"checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1), false),
			expectedError: "default cpuset is empty, but there are container assignments",
		},
		{
			name:  "static, no assignments",
			state: `{"policyName":"static","defaultCpuSet":"0-7","checksum":1}`,
			cfg:   staticConfig(cpuset.New(0, 1), false),
		},
		{
			name:  "static, with assignments",
			state: `{"policyName":"static","defaultCpuSet":"0-1,4-7","entries":{"pod1":{"c1":"2","c2":"3"}},"checksum":1}`,
			cfg:   staticConfig(cpuset.New(0, 1), false),
		},
		{
			name:  "static, reserved CPUs changed but still in the default set",
			state: `{"policyName":"static","defaultCpuSet":"0-1,4-7","entries":{"pod1":{"c1":"2","c2":"3"}},"checksum":1}`,
			cfg:   staticConfig(cpuset.New(0, 1, 4, 5), false),
		},
		{
			name:          "static, reserved CPUs assigned to a container",
			state:         `{"policyName":"static","defaultCpuSet":"0-1,4-7","entries":{"pod1":{"c1":"2","c2":"3"}},"checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1, 2), false),
			expectedError: `not all reserved CPUs "0-2" are present in the default cpuset "0-1,4-7"`,
		},
		{
			name:          "static, assignment overlaps with the default set",
			state:         `{"policyName":"static","defaultCpuSet":"0-7","entries":{"pod1":{"c1":"2-3"}},"checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1), false),
			expectedError: `cpuset "2-3" of container "c1" in pod "pod1" overlaps with the default cpuset "0-7"`,
		},
		{
			name:          "static, invalid assignment",
			state:         `{"policyName":"static","defaultCpuSet":"0-7","entries":{"pod1":{"c1":"x"}},"checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1), false),
			expectedError: `failed to parse cpuset "x" for container "c1" in pod "pod1"`,
		},
		{
			name:          "static, CPU went offline",
			state:         `{"policyName":"static","defaultCpuSet":"0-1,4-7","entries":{"pod1":{"c1":"2","c2":"3"}},"checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1), false),
			machine:       kubeletstate.Machine{OnlineCPUs: cpuset.New(0, 1, 2, 3, 4, 5, 6)},
			expectedError: `set of available CPUs "0-6" doesn't match the CPUs in the state "0-7"`,
		},
		{
			name:          "static, CPU went online",
			state:         `{"policyName":"static","defaultCpuSet":"0-7","checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1), false),
			machine:       kubeletstate.Machine{OnlineCPUs: cpuset.New(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)},
			expectedError: `set of available CPUs "0-9" doesn't match the CPUs in the state "0-7"`,
		},
		{
			name:  "strict, no assignments",
			state: `{"policyName":"static","defaultCpuSet":"2-7","checksum":1}`,
			cfg:   staticConfig(cpuset.New(0, 1), true),
		},
		{
			name:  "strict, with assignments",
			state: `{"policyName":"static","defaultCpuSet":"4-7","entries":{"pod1":{"c1":"2-3"}},"checksum":1}`,
			cfg:   staticConfig(cpuset.New(0, 1), true),
		},
		{
			name:          "strict, reserved CPUs grow",
			state:         `{"policyName":"static","defaultCpuSet":"2-7","checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1, 2, 3), true),
			expectedError: `strictly reserved CPUs "0-3" are present in the default cpuset "2-7"`,
		},
		{
			name:          "strict, reserved CPUs shrink",
			state:         `{"policyName":"static","defaultCpuSet":"2-7","checksum":1}`,
			cfg:           staticConfig(cpuset.New(0), true),
			expectedError: `set of available CPUs "1-7" doesn't match the CPUs in the state "2-7"`,
		},
		{
			name:          "strict, reserved CPUs assigned to a container",
			state:         `{"policyName":"static","defaultCpuSet":"4-7","entries":{"pod1":{"c1":"2-3"}},"checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1, 2), true),
			expectedError: `set of available CPUs "3-7" doesn't match the CPUs in the state "2-7"`,
		},
		{
			name:          "strict to non-strict",
			state:         `{"policyName":"static","defaultCpuSet":"2-7","checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1), false),
			expectedError: `not all reserved CPUs "0-1" are present in the default cpuset "2-7"`,
		},
		{
			name:          "non-strict to strict",
			state:         `{"policyName":"static","defaultCpuSet":"0-7","checksum":1}`,
			cfg:           staticConfig(cpuset.New(0, 1), true),
			expectedError: `strictly reserved CPUs "0-1" are present in the default cpuset "0-7"`,
		},
		{
			name:  "by quantity, no assignments",
			state: `{"policyName":"static","defaultCpuSet":"0-7","checksum":1}`,
			cfg:   byQuantityConfig(2, false),
		},
		{
			name:  "by quantity, with assignments",
			state: `{"policyName":"static","defaultCpuSet":"0-1,4-7","entries":{"pod1":{"c1":"2","c2":"3"}},"checksum":1}`,
			cfg:   byQuantityConfig(2, false),
		},
		{
			name:  "by quantity, reserved CPUs changed",
			state: `{"policyName":"static","defaultCpuSet":"0-1,4-7","entries":{"pod1":{"c1":"2","c2":"3"}},"checksum":1}`,
			cfg:   byQuantityConfig(4, false),
			// can't be detected, as the reserved CPUs are not recorded in the state
		},
		{
			name:          "by quantity, CPU went offline",
			state:         `{"policyName":"static","defaultCpuSet":"0-7","checksum":1}`,
			cfg:           byQuantityConfig(2, false),
			machine:       kubeletstate.Machine{OnlineCPUs: cpuset.New(0, 1, 2, 3, 4, 5, 6)},
			expectedError: `set of available CPUs "0-6" doesn't match the CPUs in the state "0-7"`,
		},
		{
			name:  "by quantity, strict, no assignments",
			state: `{"policyName":"static","defaultCpuSet":"1-3,5-7","checksum":1}`,
			cfg:   byQuantityConfig(2, true),
		},
		{
			name:  "by quantity, strict, with assignments",
			state: `{"policyName":"static","defaultCpuSet":"1,5-7","entries":{"pod1":{"c1":"2-3"}},"checksum":1}`,
			cfg:   byQuantityConfig(2, true),
		},
		{
			name:          "by quantity, strict, reserved CPUs changed",
			state:         `{"policyName":"static","defaultCpuSet":"1,5-7","entries":{"pod1":{"c1":"2-3"}},"checksum":1}`,
			cfg:           byQuantityConfig(3, true),
			expectedError: "number of strictly reserved CPUs changed from 2 to 3",
		},
		{
			name:          "by quantity, strict, CPU went online",
			state:         `{"policyName":"static","defaultCpuSet":"1-3,5-7","checksum":1}`,
			cfg:           byQuantityConfig(2, true),
			machine:       kubeletstate.Machine{OnlineCPUs: cpuset.New(0, 1, 2, 3, 4, 5, 6, 7, 8)},
			expectedError: "number of strictly reserved CPUs changed from 3 to 2",
		},
		{
			name:          "by quantity, strict, CPU went offline",
			state:         `{"policyName":"static","defaultCpuSet":"1-3,5-7","checksum":1}`,
			cfg:           byQuantityConfig(2, true),
			machine:       kubeletstate.Machine{OnlineCPUs: cpuset.New(0, 1, 2, 3, 4, 5, 6)},
			expectedError: `set of available CPUs "1-3,5-6" doesn't match the CPUs in the state "1-3,5-7"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			testMachine := test.machine
			if testMachine.OnlineCPUs.IsEmpty() {
				testMachine = machine
			}

			err := kubeletstate.ValidateCPUManagerState([]byte(test.state), test.cfg, testMachine)

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
