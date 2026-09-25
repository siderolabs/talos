// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeletstate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/kubeletstate"
)

func TestMemoryManagerConfigFromKubelet(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg kubeletconfig.KubeletConfiguration

		expected      kubeletstate.MemoryManagerConfig
		expectedError string
	}{
		{
			name: "defaults",
			expected: kubeletstate.MemoryManagerConfig{
				Policy:         kubeletconfig.NoneMemoryManagerPolicy,
				ReservedMemory: map[int]map[string]uint64{},
			},
		},
		{
			name: "static",
			cfg: kubeletconfig.KubeletConfiguration{
				MemoryManagerPolicy: kubeletconfig.StaticMemoryManagerPolicy,
				ReservedMemory: []kubeletconfig.MemoryReservation{
					{
						NumaNode: 0,
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					{
						NumaNode: 1,
						Limits: corev1.ResourceList{
							corev1.ResourceMemory:           resource.MustParse("512Mi"),
							"hugepages-1Gi":                 resource.MustParse("2Gi"),
							corev1.ResourceEphemeralStorage: resource.MustParse("0"),
						},
					},
				},
			},
			expected: kubeletstate.MemoryManagerConfig{
				Policy: kubeletconfig.StaticMemoryManagerPolicy,
				ReservedMemory: map[int]map[string]uint64{
					0: {"memory": 1 << 30},
					1: {"memory": 512 << 20, "hugepages-1Gi": 2 << 30, "ephemeral-storage": 0},
				},
			},
		},
		{
			name: "invalid quantity",
			cfg: kubeletconfig.KubeletConfiguration{
				ReservedMemory: []kubeletconfig.MemoryReservation{
					{
						NumaNode: 0,
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("-1Gi"),
						},
					},
				},
			},
			expectedError: `invalid reserved memory quantity "-1Gi" for resource "memory" on NUMA node 0`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := kubeletstate.MemoryManagerConfigFromKubelet(&test.cfg)

			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestValidateMemoryManagerState(t *testing.T) {
	t.Parallel()

	machine := kubeletstate.Machine{
		NUMANodes: cpuset.New(0, 1),
	}

	staticConfig := func(reserved map[int]map[string]uint64) kubeletstate.MemoryManagerConfig {
		return kubeletstate.MemoryManagerConfig{
			Policy:         kubeletconfig.StaticMemoryManagerPolicy,
			ReservedMemory: reserved,
		}
	}

	noneConfig := kubeletstate.MemoryManagerConfig{
		Policy: kubeletconfig.NoneMemoryManagerPolicy,
	}

	// real state: two NUMA nodes, 1Gi reserved on node 0, 2Mi hugepages on both nodes
	const twoNodesState = `{"policyName":"Static","machineState":{` +
		`"0":{"numberOfAssignments":1,"memoryMap":{` +
		`"hugepages-2Mi":{"total":4294967296,"systemReserved":0,"allocatable":4294967296,"reserved":0,"free":4294967296},` +
		`"memory":{"total":68719476736,"systemReserved":1073741824,"allocatable":63350767616,"reserved":536870912,"free":62813896704}},"cells":[0]},` +
		`"1":{"numberOfAssignments":0,"memoryMap":{` +
		`"hugepages-2Mi":{"total":4294967296,"systemReserved":0,"allocatable":4294967296,"reserved":0,"free":4294967296},` +
		`"memory":{"total":68719476736,"systemReserved":0,"allocatable":64424509440,"reserved":0,"free":64424509440}},"cells":[1]}},` +
		`"entries":{"pod1":{"c1":[{"numaAffinity":[0],"type":"memory","size":536870912}]}},"checksum":1}`

	for _, test := range []struct {
		name string

		state   string
		cfg     kubeletstate.MemoryManagerConfig
		machine kubeletstate.Machine

		expectedError string
	}{
		{
			name:  "none policy",
			state: `{"policyName":"None","machineState":{},"entries":{},"checksum":1}`,
			cfg:   noneConfig,
		},
		{
			name:          "none to static",
			state:         `{"policyName":"None","machineState":{},"entries":{},"checksum":1}`,
			cfg:           staticConfig(nil),
			expectedError: `policy changed from "None" to "Static"`,
		},
		{
			name:          "static to none",
			state:         twoNodesState,
			cfg:           noneConfig,
			expectedError: `policy changed from "Static" to "None"`,
		},
		{
			name:          "garbage",
			state:         `{"policyName":`,
			cfg:           noneConfig,
			expectedError: "failed to parse the state",
		},
		{
			name:  "static, empty state",
			state: `{"policyName":"Static","machineState":{},"entries":{},"checksum":1}`,
			cfg:   staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30}}),
		},
		{
			name:          "static, empty state with assignments",
			state:         `{"policyName":"Static","machineState":{},"entries":{"pod1":{"c1":[]}},"checksum":1}`,
			cfg:           staticConfig(nil),
			expectedError: "machine state is empty, but there are container assignments",
		},
		{
			name:  "static, unchanged",
			state: twoNodesState,
			cfg:   staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30}}),
		},
		{
			name:  "static, unchanged with explicit zero reservations",
			state: twoNodesState,
			cfg:   staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30, "hugepages-2Mi": 0}, 1: {"memory": 0}}),
		},
		{
			name:          "static, reserved memory changed",
			state:         twoNodesState,
			cfg:           staticConfig(map[int]map[string]uint64{0: {"memory": 2 << 30}}),
			expectedError: "reserved memory on NUMA node 0 changed from 1073741824 to 2147483648",
		},
		{
			name:          "static, reserved memory added on another node",
			state:         twoNodesState,
			cfg:           staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30}, 1: {"memory": 1 << 30}}),
			expectedError: "reserved memory on NUMA node 1 changed from 0 to 1073741824",
		},
		{
			name:          "static, reserved hugepages added",
			state:         twoNodesState,
			cfg:           staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30, "hugepages-2Mi": 2 << 20}}),
			expectedError: "reserved hugepages-2Mi on NUMA node 0 changed from 0 to 2097152",
		},
		{
			name:  "static, reservation of a resource not present on the node is ignored",
			state: twoNodesState,
			cfg:   staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30, "hugepages-1Gi": 1 << 30}}),
		},
		{
			name:          "static, NUMA node removed",
			state:         twoNodesState,
			cfg:           staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30}}),
			machine:       kubeletstate.Machine{NUMANodes: cpuset.New(0)},
			expectedError: `set of NUMA nodes "0" doesn't match the NUMA nodes in the state "0-1"`,
		},
		{
			name:          "static, NUMA node added",
			state:         twoNodesState,
			cfg:           staticConfig(map[int]map[string]uint64{0: {"memory": 1 << 30}}),
			machine:       kubeletstate.Machine{NUMANodes: cpuset.New(0, 1, 2)},
			expectedError: `set of NUMA nodes "0-2" doesn't match the NUMA nodes in the state "0-1"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			testMachine := test.machine
			if testMachine.NUMANodes.IsEmpty() {
				testMachine = machine
			}

			err := kubeletstate.ValidateMemoryManagerState([]byte(test.state), test.cfg, testMachine)

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
