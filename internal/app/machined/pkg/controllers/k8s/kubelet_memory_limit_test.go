// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

const (
	testPageSize       = 4096
	testMemoryCapacity = uint64(64) << 30 // M = 64 GiB
	testKubepodsLimit  = uint64(48) << 30 // C = 48 GiB
	testWorkerReserved = uint64(384) << 20
)

var testKubeletVersion = compatibility.VersionFromImageRef("ghcr.io/siderolabs/kubelet:v1.38.0-alpha.1")

func renderWithLimit(t *testing.T, extraConfig map[string]any, limit, capacity uint64) (*kubeletconfig.KubeletConfiguration, error) {
	t.Helper()

	cfgSpec := &k8s.KubeletConfigSpec{
		ClusterDNS:    []string{"10.0.0.5"},
		ClusterDomain: "cluster.local",
		ExtraConfig:   extraConfig,
	}

	return k8sctrl.NewKubeletConfiguration(cfgSpec, testKubeletVersion, k8sctrl.KubeletMemory{
		KubepodsLimit: limit,
		Capacity:      capacity,
		Reserved:      testWorkerReserved,
		PageSize:      testPageSize,
	})
}

func TestKubepodsMemoryLimitSystemReserved(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		extraConfig map[string]any
		limit       uint64
		capacity    uint64

		expectedSystemReserved map[string]string
	}{
		{
			name:     "no user map keeps Talos defaults",
			limit:    testKubepodsLimit,
			capacity: testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10),
			},
		},
		{
			name:        "explicit empty user map keeps Talos defaults",
			extraConfig: map[string]any{"systemReserved": map[string]any{}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10),
			},
		},
		{
			name:        "user map without memory keeps only user entries",
			extraConfig: map[string]any{"systemReserved": map[string]any{"cpu": "1", "pid": "2000"}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":    "1",
				"pid":    "2000",
				"memory": strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10),
			},
		},
		{
			name:        "kubeReserved without memory is kept",
			extraConfig: map[string]any{"kubeReserved": map[string]any{"cpu": "100m"}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10),
			},
		},
		{
			name:     "limit equal to capacity yields zero reservation",
			limit:    testMemoryCapacity,
			capacity: testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            "0",
			},
		},
		{
			name:     "limit is rounded down to the page size",
			limit:    testKubepodsLimit + testPageSize - 1,
			capacity: testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10),
			},
		},
		{
			name:     "limit below the Talos default reservation is accepted",
			limit:    testMemoryCapacity - (128 << 20),
			capacity: testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            strconv.FormatUint(128<<20, 10),
			},
		},
		{
			name:        "eviction and hugepages do not change the reservation",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "2Gi", "nodefs.available": "10%"}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,

			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, err := renderWithLimit(t, tt.extraConfig, tt.limit, tt.capacity)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedSystemReserved, config.SystemReserved)
		})
	}
}

func TestKubepodsMemoryLimitEffectiveTarget(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		extraConfig map[string]any
		limit       uint64

		expectedKubepodsMax uint64
	}{
		{name: "aligned", limit: testKubepodsLimit, expectedKubepodsMax: testKubepodsLimit},
		{name: "sub-page remainder", limit: testKubepodsLimit + 1, expectedKubepodsMax: testKubepodsLimit},
		{
			name:                "one page with eviction disabled",
			extraConfig:         map[string]any{"evictionHard": map[string]any{"memory.available": "0%"}},
			limit:               testPageSize,
			expectedKubepodsMax: testPageSize,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, err := renderWithLimit(t, tt.extraConfig, tt.limit, testMemoryCapacity)
			require.NoError(t, err)

			systemReserved, err := strconv.ParseUint(config.SystemReserved["memory"], 10, 64)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedKubepodsMax, testMemoryCapacity-systemReserved)
		})
	}
}

func TestKubepodsMemoryLimitRejected(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		extraConfig map[string]any
		limit       uint64
		capacity    uint64

		expectedErr string
	}{
		{
			name:        "limit above capacity",
			limit:       testMemoryCapacity + testPageSize,
			capacity:    testMemoryCapacity,
			expectedErr: `kubepods memory limit 68719480832 bytes exceeds the host memory capacity 68719476736 bytes`,
		},
		{
			name:        "kubeReserved memory conflicts",
			extraConfig: map[string]any{"kubeReserved": map[string]any{"memory": "1Gi"}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,
			expectedErr: `kubelet configuration field "kubeReserved.memory" conflicts with the kubepods memory limit: ` +
				`the derived "systemReserved.memory" already holds all of the host memory beyond the limit`,
		},
		{
			name:        "limit below the page size",
			limit:       testPageSize - 1,
			capacity:    testMemoryCapacity,
			expectedErr: "invalid kubepods memory limit: memory limit 4095 bytes is smaller than the page size of 4096 bytes",
		},
		{
			name:        "limit at the kernel unlimited sentinel",
			limit:       math.MaxInt64,
			capacity:    math.MaxInt64,
			expectedErr: "invalid kubepods memory limit: memory limit 9223372036854775807 bytes is treated as unlimited by the kernel (limit must be below 9223372036854771712 bytes)",
		},
		{
			name:        "limit above int64",
			limit:       math.MaxUint64,
			capacity:    testMemoryCapacity,
			expectedErr: "invalid kubepods memory limit: memory limit 18446744073709551615 bytes exceeds the maximum of 9223372036854775807 bytes",
		},
		{
			name:        "zero capacity",
			limit:       testKubepodsLimit,
			capacity:    0,
			expectedErr: "host memory capacity 0 bytes is outside the kubelet's range",
		},
		{
			name:        "capacity above int64",
			limit:       testKubepodsLimit,
			capacity:    math.MaxUint64,
			expectedErr: "host memory capacity 18446744073709551615 bytes is outside the kubelet's range",
		},
		{
			name:        "limit below the default eviction threshold",
			limit:       (100 << 20) - testPageSize,
			capacity:    testMemoryCapacity,
			expectedErr: `kubepods memory limit 104853504 bytes is below the hard eviction threshold "evictionHard.memory.available" of 100Mi (104857600 bytes): the kubelet would refuse to start`,
		},
		{
			name:        "limit below an explicit eviction threshold",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "49Gi"}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,
			expectedErr: `kubepods memory limit 51539607552 bytes is below the hard eviction threshold "evictionHard.memory.available" of 49Gi (52613349376 bytes): the kubelet would refuse to start`,
		},
		{
			name:        "limit below a percentage eviction threshold",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "80%"}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,
			expectedErr: `kubepods memory limit 51539607552 bytes is below the hard eviction threshold "evictionHard.memory.available" of 53687092Ki (54975582208 bytes): the kubelet would refuse to start`,
		},
		{
			name:        "systemReserved memory conflicts before defaults",
			extraConfig: map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}},
			limit:       testKubepodsLimit,
			capacity:    testMemoryCapacity,
			expectedErr: `kubelet configuration field "systemReserved.memory" conflicts with the kubepods memory limit: Talos derives it from the limit`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := renderWithLimit(t, tt.extraConfig, tt.limit, tt.capacity)
			require.Error(t, err)

			assert.EqualError(t, err, tt.expectedErr)
		})
	}
}

func TestKubepodsMemoryLimitExtraArgsRejected(t *testing.T) {
	t.Parallel()

	cfgSpec := &k8s.KubeletConfigSpec{
		ClusterDNS:    []string{"10.0.0.5"},
		ClusterDomain: "cluster.local",
		ExtraArgs:     map[string]k8s.ArgValues{"system-reserved": {Values: []string{"cpu=500m"}}, "node-labels": {Values: []string{"a=b"}}},
	}

	_, err := k8sctrl.NewKubeletConfiguration(cfgSpec, testKubeletVersion, k8sctrl.KubeletMemory{
		KubepodsLimit: testKubepodsLimit,
		Capacity:      testMemoryCapacity,
		Reserved:      testWorkerReserved,
		PageSize:      testPageSize,
	})
	assert.EqualError(t, err, `kubelet argument "system-reserved" conflicts with the kubepods memory limit: use the "systemReserved" configuration field instead`)
}

func TestSystemReservedMemory(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		extraConfig map[string]any
		reserved    uint64

		expectedSystemReserved map[string]string
	}{
		{
			name:     "defaults",
			reserved: testWorkerReserved,
			expectedSystemReserved: map[string]string{
				"cpu":               constants.KubeletSystemReservedCPU,
				"pid":               constants.KubeletSystemReservedPid,
				"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
				"memory":            "402653184",
			},
		},
		{
			name:                   "user map without memory gets the reservation",
			extraConfig:            map[string]any{"systemReserved": map[string]any{"cpu": "1"}, "kubeReserved": map[string]any{"memory": "1Gi"}},
			reserved:               2 << 30,
			expectedSystemReserved: map[string]string{"cpu": "1", "memory": "2147483648"},
		},
		{
			name:                   "user memory and conflicting settings are accepted without a kubepods limit",
			extraConfig:            map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}, "cgroupsPerQOS": false},
			reserved:               testWorkerReserved,
			expectedSystemReserved: map[string]string{"memory": "1Gi"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfgSpec := &k8s.KubeletConfigSpec{
				ClusterDNS:    []string{"10.0.0.5"},
				ClusterDomain: "cluster.local",
				ExtraConfig:   tt.extraConfig,
			}

			config, err := k8sctrl.NewKubeletConfiguration(cfgSpec, testKubeletVersion, k8sctrl.KubeletMemory{Reserved: tt.reserved, PageSize: testPageSize})
			require.NoError(t, err)

			assert.Equal(t, tt.expectedSystemReserved, config.SystemReserved)
		})
	}
}

// TestHardMemoryEvictionThreshold pins the narrow parser to kubelet v1.38.0-alpha.1:
// cmd/kubelet/app/server.go loadConfigFile (defaults and mergeDefaultEvictionSettings),
// pkg/kubelet/eviction/helpers.go parseThresholdStatement/parsePercentage and
// pkg/kubelet/eviction/api/types.go GetThresholdQuantity (int64(float64(capacity) * float64(float32 percentage))).
func TestHardMemoryEvictionThreshold(t *testing.T) {
	t.Parallel()

	const capacity = testMemoryCapacity

	for _, tt := range []struct {
		name        string
		extraConfig map[string]any

		// the largest limit which must be rejected and the smallest which must be accepted; zero skips the check
		rejectedLimit uint64
		acceptedLimit uint64
		expectedErr   string
	}{
		{
			name:          "unset uses the 100Mi default",
			rejectedLimit: (100 << 20) - testPageSize,
			acceptedLimit: 100 << 20,
		},
		{
			name:          "explicit empty map uses the 100Mi default",
			extraConfig:   map[string]any{"evictionHard": map[string]any{}},
			rejectedLimit: (100 << 20) - testPageSize,
			acceptedLimit: 100 << 20,
		},
		{
			name:          "other signals only disable the memory threshold",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"nodefs.available": "10%"}},
			acceptedLimit: testPageSize,
		},
		{
			name:          "other signals only with merge fall back to 100Mi",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"nodefs.available": "10%"}, "mergeDefaultEvictionSettings": true},
			rejectedLimit: (100 << 20) - testPageSize,
			acceptedLimit: 100 << 20,
		},
		{
			name:          "other signals only with merge disabled",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"nodefs.available": "10%"}, "mergeDefaultEvictionSettings": false},
			acceptedLimit: testPageSize,
		},
		{
			name:          "explicit memory threshold wins over merge",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "1Gi"}, "mergeDefaultEvictionSettings": true},
			rejectedLimit: (1 << 30) - testPageSize,
			acceptedLimit: 1 << 30,
		},
		{
			name:          "quantity",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "1Gi"}},
			rejectedLimit: (1 << 30) - testPageSize,
			acceptedLimit: 1 << 30,
		},
		{
			name:          "decimal quantity",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "1.5G"}},
			rejectedLimit: 1_499_998_208,
			acceptedLimit: 1_500_000_256,
		},
		{
			name:          "fractional quantity compares exactly",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "4096.5"}},
			rejectedLimit: testPageSize,
			acceptedLimit: 2 * testPageSize,
		},
		{
			name:          "percentage of capacity",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "25%"}},
			rejectedLimit: (16 << 30) - testPageSize,
			acceptedLimit: 16 << 30,
		},
		{
			name:        "fractional percentage uses float32 arithmetic",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "33.3%"}},
			// int64(float64(64Gi) * float64(float32(33.3)/100)) = 22883586048, not the exact 22883584000
			rejectedLimit: 22883581952,
			acceptedLimit: 22883586048,
		},
		{
			name:        "percentage uses the float32 value of the fraction",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "80%"}},
			// int64(float64(64Gi) * float64(float32(0.8))) = 54975582208, not the exact 54975581388
			rejectedLimit: 54975578112,
			acceptedLimit: 54975582208,
		},
		{
			name:          "0% disables the threshold",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "0%"}},
			acceptedLimit: testPageSize,
		},
		{
			name:          "100% disables the threshold",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "100%"}},
			acceptedLimit: testPageSize,
		},
		{
			name:          "0.0% is not the literal 0% and yields a zero threshold",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "0.0%"}},
			acceptedLimit: testPageSize,
		},
		{
			name:          "100.0% is not the literal 100% and reserves the whole capacity",
			extraConfig:   map[string]any{"evictionHard": map[string]any{"memory.available": "100.0%"}},
			rejectedLimit: capacity - testPageSize,
			acceptedLimit: capacity,
		},
		{
			name:        "zero quantity",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "0"}},
			expectedErr: `invalid kubelet configuration field "evictionHard.memory.available" "0": eviction threshold must be positive`,
		},
		{
			name:        "negative quantity",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "-1Gi"}},
			expectedErr: `invalid kubelet configuration field "evictionHard.memory.available" "-1Gi": eviction threshold must be positive`,
		},
		{
			name:        "negative percentage",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "-5%"}},
			expectedErr: `invalid kubelet configuration field "evictionHard.memory.available" "-5%": eviction percentage threshold must be >= 0%`,
		},
		{
			name:        "percentage above 100",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "150%"}},
			expectedErr: `invalid kubelet configuration field "evictionHard.memory.available" "150%": eviction percentage threshold must be <= 100%`,
		},
		{
			name:        "malformed percentage",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "ten%"}},
			expectedErr: `invalid kubelet configuration field "evictionHard.memory.available" "ten%": strconv.ParseFloat: parsing "ten": invalid syntax`,
		},
		{
			name:        "NaN percentage",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "NaN%"}},
			expectedErr: `invalid kubelet configuration field "evictionHard.memory.available" "NaN%": eviction percentage threshold is not a number`,
		},
		{
			name:        "malformed quantity",
			extraConfig: map[string]any{"evictionHard": map[string]any{"memory.available": "lots"}},
			expectedErr: `invalid kubelet configuration field "evictionHard.memory.available" "lots": quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.expectedErr != "" {
				_, err := renderWithLimit(t, tt.extraConfig, testKubepodsLimit, capacity)
				assert.EqualError(t, err, tt.expectedErr)

				return
			}

			if tt.rejectedLimit != 0 {
				_, err := renderWithLimit(t, tt.extraConfig, tt.rejectedLimit, capacity)
				assert.ErrorContains(t, err, `is below the hard eviction threshold "evictionHard.memory.available"`)
			}

			config, err := renderWithLimit(t, tt.extraConfig, tt.acceptedLimit, capacity)
			require.NoError(t, err)

			assert.Equal(t, strconv.FormatUint(capacity-tt.acceptedLimit, 10), config.SystemReserved["memory"])
		})
	}
}
