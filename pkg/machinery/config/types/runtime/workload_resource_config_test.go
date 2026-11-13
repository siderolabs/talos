// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

//go:embed testdata/workloadresourceconfig.yaml
var expectedWorkloadResourceConfigDocument []byte

func workloadMemoryRoot(limit string) *runtime.WorkloadResourceRoot {
	return &runtime.WorkloadResourceRoot{
		MemoryConfig: &runtime.WorkloadMemoryResource{MemoryLimit: meta.MustPercentageSize(limit)},
	}
}

func acceptedWorkloadResourceConfig() *runtime.WorkloadResourceConfigV1Alpha1 {
	cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
	cfg.ReservedConfig = &runtime.WorkloadReserved{MemoryConfig: meta.MustByteSize("2GiB")}
	cfg.KubepodsConfig = workloadMemoryRoot("50%")
	cfg.TalosContainersConfig = workloadMemoryRoot("10%")
	cfg.VirtualMachinesConfig = workloadMemoryRoot("40%")

	return cfg
}

func TestWorkloadResourceConfigMarshalStability(t *testing.T) {
	t.Parallel()

	marshaled, err := encoder.NewEncoder(acceptedWorkloadResourceConfig(), encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	assert.Equal(t, string(expectedWorkloadResourceConfigDocument), string(marshaled))

	provider, err := configloader.NewFromBytes(marshaled)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)
	assert.Equal(t, acceptedWorkloadResourceConfig(), docs[0])

	partial := runtime.NewWorkloadResourceConfigV1Alpha1()
	partial.VirtualMachinesConfig = workloadMemoryRoot("80%")

	marshaled, err = encoder.NewEncoder(partial, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)
	assert.Equal(t, "apiVersion: v1alpha1\nkind: WorkloadResourceConfig\nvirtualMachines:\n    memory:\n        limit: 80%\n", string(marshaled))
}

func TestWorkloadResourceConfigRejectedOnLoad(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		document string
		expected string
	}{
		{"unknown root", "system:\n    memory:\n        limit: 10%\n", "system"},
		{"size instead of a percentage", "kubepods:\n    memory:\n        limit: 16GiB\n", "percentage must end with '%'"},
		{"above 100", "kubepods:\n    memory:\n        limit: 101%\n", "percentage must be between 0 and 100"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := configloader.NewFromBytes([]byte("apiVersion: v1alpha1\nkind: WorkloadResourceConfig\n" + test.document))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expected)
		})
	}
}

func TestWorkloadResourceConfigLoadedValidation(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: WorkloadResourceConfig
kubepods:
    memory:
        limit: 0%
taloscontainers:
    memory: {}
virtualMachines:
    memory:
        limit: -10%
`))
	require.NoError(t, err)

	_, err = provider.ValidateAsClient(validationMode{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WorkloadResourceConfig: ")
	assert.Contains(t, err.Error(), "kubepods.memory.limit must be greater than zero")
	assert.Contains(t, err.Error(), "taloscontainers.memory.limit is required")
	assert.Contains(t, err.Error(), "virtualMachines.memory.limit cannot be negative")
}

func TestWorkloadResourceConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.WorkloadResourceConfigV1Alpha1

		expectedError string
	}{
		{
			name: "reserved only",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.ReservedConfig = &runtime.WorkloadReserved{MemoryConfig: meta.MustByteSize("2GiB")}

				return cfg
			},
		},
		{
			name: "accepted",
			cfg:  acceptedWorkloadResourceConfig,
		},
		{
			name: "single root",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.TalosContainersConfig = workloadMemoryRoot("100%")

				return cfg
			},
		},
		{
			name: "present roots without a limit",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.KubepodsConfig = &runtime.WorkloadResourceRoot{}
				cfg.VirtualMachinesConfig = &runtime.WorkloadResourceRoot{MemoryConfig: &runtime.WorkloadMemoryResource{}}

				return cfg
			},

			expectedError: "kubepods.memory.limit is required\nvirtualMachines.memory.limit is required",
		},
		{
			name: "above 100 in total",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.KubepodsConfig = workloadMemoryRoot("60%")
				cfg.VirtualMachinesConfig = workloadMemoryRoot("50%")

				return cfg
			},

			expectedError: "memory limits add up to 110%, more than 100%",
		},
		{
			name: "zero reserved",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := acceptedWorkloadResourceConfig()
				cfg.ReservedConfig = &runtime.WorkloadReserved{MemoryConfig: meta.MustByteSize("0")}

				return cfg
			},

			expectedError: "reserved.memory must be greater than zero",
		},
		{
			name: "fractional, zero and negative limits, negative reserved",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.ReservedConfig = &runtime.WorkloadReserved{MemoryConfig: meta.MustByteSize("-1GiB")}
				cfg.KubepodsConfig = workloadMemoryRoot("12.5%")
				cfg.TalosContainersConfig = workloadMemoryRoot("-10%")
				cfg.VirtualMachinesConfig = workloadMemoryRoot("0%")

				return cfg
			},

			expectedError: "reserved.memory cannot be negative\n" +
				"kubepods.memory.limit must be a whole percentage\n" +
				"taloscontainers.memory.limit cannot be negative\n" +
				"virtualMachines.memory.limit must be greater than zero",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Empty(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkloadResourceConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := acceptedWorkloadResourceConfig()

	assert.Equal(t, optional.Some(uint64(2<<30)), cfg.MemoryReserved())
	assert.Equal(t, uint64(50), cfg.KubepodsMemoryPercent())
	assert.Equal(t, uint64(10), cfg.TalosContainersMemoryPercent())
	assert.Equal(t, uint64(40), cfg.VirtualMachinesMemoryPercent())

	partial := runtime.NewWorkloadResourceConfigV1Alpha1()
	partial.KubepodsConfig = &runtime.WorkloadResourceRoot{}
	partial.VirtualMachinesConfig = workloadMemoryRoot("-10%")

	assert.False(t, partial.MemoryReserved().IsPresent())
	assert.Zero(t, partial.KubepodsMemoryPercent())
	assert.Zero(t, partial.TalosContainersMemoryPercent())
	assert.Zero(t, partial.VirtualMachinesMemoryPercent())

	// An explicit zero is kept for Validate to reject.
	partial.ReservedConfig = &runtime.WorkloadReserved{MemoryConfig: meta.MustByteSize("0")}
	assert.Equal(t, optional.Some(uint64(0)), partial.MemoryReserved())
}

func TestWorkloadResourceConfigClone(t *testing.T) {
	t.Parallel()

	cfg := acceptedWorkloadResourceConfig()

	clone, ok := cfg.Clone().(*runtime.WorkloadResourceConfigV1Alpha1)
	require.True(t, ok)
	assert.Equal(t, cfg, clone)

	clone.ReservedConfig = &runtime.WorkloadReserved{MemoryConfig: meta.MustByteSize("4GiB")}
	clone.KubepodsConfig.MemoryConfig.MemoryLimit = meta.MustPercentageSize("1%")
	clone.VirtualMachinesConfig.MemoryConfig = nil

	assert.Equal(t, optional.Some(uint64(2<<30)), cfg.MemoryReserved())
	assert.Equal(t, uint64(50), cfg.KubepodsMemoryPercent())
	assert.Equal(t, uint64(40), cfg.VirtualMachinesMemoryPercent())
}

func TestWorkloadResourceConfigMerge(t *testing.T) {
	t.Parallel()

	patch := runtime.NewWorkloadResourceConfigV1Alpha1()
	patch.KubepodsConfig = workloadMemoryRoot("30%")

	left, err := container.New(acceptedWorkloadResourceConfig())
	require.NoError(t, err)

	right, err := container.New(patch)
	require.NoError(t, err)

	merged, err := configpatcher.StrategicMerge(left, configpatcher.NewStrategicMergePatch(right))
	require.NoError(t, err)

	documents := merged.Documents()
	require.Len(t, documents, 1)

	cfg, ok := documents[0].(*runtime.WorkloadResourceConfigV1Alpha1)
	require.True(t, ok)

	assert.Equal(t, optional.Some(uint64(2<<30)), cfg.MemoryReserved())
	assert.Equal(t, uint64(30), cfg.KubepodsMemoryPercent())
	assert.Equal(t, uint64(10), cfg.TalosContainersMemoryPercent())
	assert.Equal(t, uint64(40), cfg.VirtualMachinesMemoryPercent())
}

func TestWorkloadResourceConfigSingleton(t *testing.T) {
	t.Parallel()

	_, err := container.New(acceptedWorkloadResourceConfig(), acceptedWorkloadResourceConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate document: WorkloadResourceConfig")
}
