// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeletstate_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/kubeletstate"
)

func TestDiscoverMachine(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("sysfs is only available on Linux")
	}

	machine, err := kubeletstate.DiscoverMachine()
	require.NoError(t, err)

	assert.False(t, machine.OnlineCPUs.IsEmpty())
	assert.False(t, machine.NUMANodes.IsEmpty())
}

func TestCleanup(t *testing.T) {
	t.Parallel()

	const (
		cpuManagerState    = "cpu_manager_state"
		memoryManagerState = "memory_manager_state"

		staticCPUState    = `{"policyName":"static","defaultCpuSet":"1,3-15,17,19-31","checksum":1902567528}`
		staticMemoryState = `{"policyName":"Static","machineState":{"0":{"numberOfAssignments":0,"memoryMap":{` +
			`"memory":{"total":68719476736,"systemReserved":1073741824,"allocatable":67645734912,"reserved":0,"free":67645734912}},"cells":[0]}},"entries":{},"checksum":1}`
	)

	machine := kubeletstate.Machine{
		OnlineCPUs: cpuset.New(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31),
		NUMANodes:  cpuset.New(0),
	}

	staticConfig := func(reservedCPUs, reservedMemory string) *kubeletconfig.KubeletConfiguration {
		return &kubeletconfig.KubeletConfiguration{
			CPUManagerPolicy:        kubeletstate.CPUManagerPolicyStatic,
			CPUManagerPolicyOptions: map[string]string{"strict-cpu-reservation": "true"},
			ReservedSystemCPUs:      reservedCPUs,
			MemoryManagerPolicy:     kubeletconfig.StaticMemoryManagerPolicy,
			ReservedMemory: []kubeletconfig.MemoryReservation{
				{
					NumaNode: 0,
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse(reservedMemory),
					},
				},
			},
			SystemReserved: map[string]string{"memory": reservedMemory},
		}
	}

	for _, test := range []struct {
		name string

		states map[string]string
		cfg    *kubeletconfig.KubeletConfiguration

		expectedRemoved []string
	}{
		{
			name: "no state",
			cfg:  staticConfig("0,2,16,18", "1Gi"),
		},
		{
			name:   "unchanged",
			states: map[string]string{cpuManagerState: staticCPUState, memoryManagerState: staticMemoryState},
			cfg:    staticConfig("0,2,16,18", "1Gi"),
		},
		{
			name:            "policy changed",
			states:          map[string]string{cpuManagerState: staticCPUState, memoryManagerState: staticMemoryState},
			cfg:             &kubeletconfig.KubeletConfiguration{},
			expectedRemoved: []string{cpuManagerState, memoryManagerState},
		},
		{
			name:            "reserved CPUs changed",
			states:          map[string]string{cpuManagerState: staticCPUState, memoryManagerState: staticMemoryState},
			cfg:             staticConfig("0,2,4,6,16,18,20,22", "1Gi"),
			expectedRemoved: []string{cpuManagerState},
		},
		{
			name:            "reserved memory changed",
			states:          map[string]string{cpuManagerState: staticCPUState, memoryManagerState: staticMemoryState},
			cfg:             staticConfig("0,2,16,18", "2Gi"),
			expectedRemoved: []string{memoryManagerState},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			for name, state := range test.states {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(state), 0o600))
			}

			require.NoError(t, kubeletstate.Cleanup(dir, test.cfg, machine, zaptest.NewLogger(t)))

			for name := range test.states {
				_, err := os.Stat(filepath.Join(dir, name))

				if slices.Contains(test.expectedRemoved, name) {
					assert.ErrorIs(t, err, os.ErrNotExist, "expected %s to be removed", name)
				} else {
					assert.NoError(t, err, "expected %s to be kept", name)
				}
			}

			// the second run with the same config should never remove anything
			for name, state := range test.states {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(state), 0o600))
			}

			require.NoError(t, kubeletstate.Cleanup(dir, test.cfg, machine, zaptest.NewLogger(t)))
		})
	}

	t.Run("missing directory", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, kubeletstate.Cleanup(filepath.Join(t.TempDir(), "missing"), staticConfig("0,2,16,18", "1Gi"), machine, zaptest.NewLogger(t)))
	})
}
