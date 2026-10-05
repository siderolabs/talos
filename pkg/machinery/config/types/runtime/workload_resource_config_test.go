// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
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
		MemoryConfig: &runtime.WorkloadMemoryResource{MemoryLimit: meta.MustByteSize(limit)},
	}
}

func acceptedWorkloadResourceConfig() *runtime.WorkloadResourceConfigV1Alpha1 {
	cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
	cfg.KubepodsConfig = workloadMemoryRoot("16GiB")
	cfg.TalosContainersConfig = workloadMemoryRoot("4GiB")
	cfg.VirtualMachinesConfig = workloadMemoryRoot("32GiB")

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
	partial.VirtualMachinesConfig = workloadMemoryRoot("32GiB")

	marshaled, err = encoder.NewEncoder(partial, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)
	assert.Equal(t, "apiVersion: v1alpha1\nkind: WorkloadResourceConfig\nvirtualMachines:\n    memory:\n        limit: 32GiB\n", string(marshaled))
}

func TestWorkloadResourceConfigUnknownRootRejected(t *testing.T) {
	t.Parallel()

	_, err := configloader.NewFromBytes([]byte("apiVersion: v1alpha1\nkind: WorkloadResourceConfig\nsystem:\n    memory:\n        limit: 1GiB\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system")
}

func TestWorkloadResourceConfigLoadedValidation(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: WorkloadResourceConfig
kubepods:
    memory:
        limit: 0
taloscontainers:
    memory: {}
virtualMachines:
    memory:
        limit: -1GiB
`))
	require.NoError(t, err)

	_, err = provider.ValidateAsClient(validationMode{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WorkloadResourceConfig: ")
	assert.Contains(t, err.Error(), "kubepods.memory.limit must be greater than zero")
	assert.Contains(t, err.Error(), "taloscontainers.memory.limit is required")
	assert.Contains(t, err.Error(), "virtualMachines.memory.limit cannot be negative")
}

func TestWorkloadResourceConfigRuntimeValidation(t *testing.T) {
	t.Parallel()

	pageSize := uint64(os.Getpagesize())
	sentinel := (uint64(math.MaxInt64) / pageSize) * pageSize

	for _, root := range []string{"kubepods", "taloscontainers", "virtualMachines"} {
		for _, test := range []struct {
			name    string
			limit   uint64
			invalid bool
		}{
			{"subpage", 1, true},
			{"sentinel", sentinel, true},
			{"max-int64", math.MaxInt64, true},
			{"one-page", pageSize, false},
			{"unaligned", pageSize + 1, false},
			{"largest-finite", sentinel - pageSize, false},
			{"largest-finite-rounded", sentinel - 1, false},
		} {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				t.Parallel()

				provider, err := configloader.NewFromBytes(fmt.Appendf(nil, `apiVersion: v1alpha1
kind: WorkloadResourceConfig
%s:
  memory:
    limit: %dB
---
apiVersion: v1alpha1
kind: ResolverConfig
hostDNS:
  enabled: true
  forwardKubeDNSToHost: true
`, root, test.limit))
				require.NoError(t, err)

				// Client validation must not use the client's page size for a remote node.
				_, err = provider.ValidateAsClient(validationMode{})
				require.NoError(t, err)

				for _, inContainer := range []bool{false, true} {
					_, err = provider.ValidateAtRuntime(t.Context(), state.WrapCore(namespaced.NewState(inmem.Build)), workloadValidationMode{inContainer: inContainer})
					if test.invalid {
						require.Error(t, err, "expected error got nil")
						assert.Contains(t, err.Error(), root+".memory.limit")
					} else {
						require.NoError(t, err)
					}
				}
			})
		}
	}

	t.Run("all violations", func(t *testing.T) {
		t.Parallel()

		cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
		cfg.KubepodsConfig = workloadMemoryRoot("1B")
		cfg.TalosContainersConfig = workloadMemoryRoot(fmt.Sprintf("%dB", sentinel))
		cfg.VirtualMachinesConfig = workloadMemoryRoot(fmt.Sprintf("%dB", int64(math.MaxInt64)))
		provider, err := container.New(cfg)
		require.NoError(t, err)
		_, err = provider.ValidateAtRuntime(t.Context(), state.WrapCore(namespaced.NewState(inmem.Build)), validationMode{})
		require.Error(t, err, "expected error got nil")

		for _, root := range []string{"kubepods", "taloscontainers", "virtualMachines"} {
			assert.Contains(t, err.Error(), root+".memory.limit")
		}
	})
}

func TestWorkloadResourceRuntimeValidationDefersStaticErrors(t *testing.T) {
	t.Parallel()

	for _, root := range []*runtime.WorkloadResourceRoot{
		nil,
		{},
		{MemoryConfig: &runtime.WorkloadMemoryResource{}},
		workloadMemoryRoot("-1B"),
		workloadMemoryRoot("0B"),
		workloadMemoryRoot("9223372036854775808B"),
	} {
		cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
		cfg.KubepodsConfig = root
		cfg.TalosContainersConfig = root
		cfg.VirtualMachinesConfig = root
		_, err := cfg.RuntimeValidate(t.Context(), nil, validationMode{})
		require.NoError(t, err)
	}
}

type workloadValidationMode struct {
	validationMode
	inContainer bool
}

func (m workloadValidationMode) InContainer() bool { return m.inContainer }

func TestWorkloadResourceConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.WorkloadResourceConfigV1Alpha1

		expectedError string
	}{
		{
			name: "empty",
			cfg:  runtime.NewWorkloadResourceConfigV1Alpha1,

			expectedError: "at least one root must be limited",
		},
		{
			name: "accepted",
			cfg:  acceptedWorkloadResourceConfig,
		},
		{
			name: "single root",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.TalosContainersConfig = workloadMemoryRoot("1")

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
			name: "zero, negative and oversized limits",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.KubepodsConfig = workloadMemoryRoot("0")
				cfg.TalosContainersConfig = workloadMemoryRoot("-4GiB")
				cfg.VirtualMachinesConfig = workloadMemoryRoot("9223372036854775808")

				return cfg
			},

			expectedError: "kubepods.memory.limit must be greater than zero\n" +
				"taloscontainers.memory.limit cannot be negative\n" +
				"virtualMachines.memory.limit cannot be greater than 9223372036854775807 bytes",
		},
		{
			name: "largest accepted limit",
			cfg: func() *runtime.WorkloadResourceConfigV1Alpha1 {
				cfg := runtime.NewWorkloadResourceConfigV1Alpha1()
				cfg.VirtualMachinesConfig = workloadMemoryRoot("9223372036854775807")

				return cfg
			},
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

	assert.Equal(t, uint64(16<<30), cfg.KubepodsMemoryLimit())
	assert.Equal(t, uint64(4<<30), cfg.TalosContainersMemoryLimit())
	assert.Equal(t, uint64(32<<30), cfg.VirtualMachinesMemoryLimit())

	partial := runtime.NewWorkloadResourceConfigV1Alpha1()
	partial.KubepodsConfig = &runtime.WorkloadResourceRoot{}
	partial.VirtualMachinesConfig = workloadMemoryRoot("-1GiB")

	assert.Zero(t, partial.KubepodsMemoryLimit())
	assert.Zero(t, partial.TalosContainersMemoryLimit())
	assert.Zero(t, partial.VirtualMachinesMemoryLimit())
}

func TestWorkloadResourceConfigClone(t *testing.T) {
	t.Parallel()

	cfg := acceptedWorkloadResourceConfig()

	clone, ok := cfg.Clone().(*runtime.WorkloadResourceConfigV1Alpha1)
	require.True(t, ok)
	assert.Equal(t, cfg, clone)

	clone.KubepodsConfig.MemoryConfig.MemoryLimit = meta.MustByteSize("1GiB")
	clone.VirtualMachinesConfig.MemoryConfig = nil

	assert.Equal(t, uint64(16<<30), cfg.KubepodsMemoryLimit())
	assert.Equal(t, uint64(32<<30), cfg.VirtualMachinesMemoryLimit())
}

func TestWorkloadResourceConfigMerge(t *testing.T) {
	t.Parallel()

	patch := runtime.NewWorkloadResourceConfigV1Alpha1()
	patch.KubepodsConfig = workloadMemoryRoot("8GiB")

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

	assert.Equal(t, uint64(8<<30), cfg.KubepodsMemoryLimit())
	assert.Equal(t, uint64(4<<30), cfg.TalosContainersMemoryLimit())
	assert.Equal(t, uint64(32<<30), cfg.VirtualMachinesMemoryLimit())
}

func TestWorkloadResourceConfigSingleton(t *testing.T) {
	t.Parallel()

	_, err := container.New(acceptedWorkloadResourceConfig(), acceptedWorkloadResourceConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate document: WorkloadResourceConfig")
}
