// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//go:embed testdata/cpupartitionconfig.yaml
var expectedCPUPartitionConfigDocument []byte

func acceptedCPUPartitionConfig() *runtime.CPUPartitionConfigV1Alpha1 {
	cfg := runtime.NewCPUPartitionConfigV1Alpha1()
	cfg.InitConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.SystemConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.PodRuntimeConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: "2-3"}
	cfg.TalosContainersConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-1"}
	cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
		RootCPUs: "4-7",
		SlicesConfig: []runtime.CPUPartitionSlice{
			{SliceName: "database", SliceCPUs: "4-5", SliceExclusive: new(true)},
		},
	}

	return cfg
}

func TestCPUPartitionConfigMarshalStability(t *testing.T) {
	t.Parallel()

	marshaled, err := encoder.NewEncoder(acceptedCPUPartitionConfig(), encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	assert.Equal(t, string(expectedCPUPartitionConfigDocument), string(marshaled))

	provider, err := configloader.NewFromBytes(marshaled)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)
	assert.Equal(t, acceptedCPUPartitionConfig(), docs[0])

	partial := runtime.NewCPUPartitionConfigV1Alpha1()
	partial.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: "2-3"}

	marshaled, err = encoder.NewEncoder(partial, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)
	assert.Equal(t, "apiVersion: v1alpha1\nkind: CPUPartitionConfig\nkubepods:\n    cpus: 2-3\n", string(marshaled))
}

func TestCPUPartitionConfigUnknownRootRejected(t *testing.T) {
	t.Parallel()

	_, err := configloader.NewFromBytes([]byte("apiVersion: v1alpha1\nkind: CPUPartitionConfig\nkubelet:\n    cpus: 0-1\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubelet")
}

func TestCPUPartitionConfigKubernetesOnlyLoad(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: CPUPartitionConfig
init:
    cpus: 1,0
system:
    cpus: 0-1
podruntime:
    cpus: 0-1
kubepods:
    cpus: 2-7,3
`))
	require.NoError(t, err)

	partition := provider.CPUPartitionConfig()
	require.NotNil(t, partition)
	assert.Equal(t, map[config.CPUPartitionRoot]string{
		constants.CgroupInit:           "0-1",
		constants.CgroupSystem:         "0-1",
		constants.CgroupPodRuntimeRoot: "0-1",
		constants.CgroupKubepods:       "2-7",
	}, partition.Roots())
	assert.Empty(t, partition.Slices())
	assert.Empty(t, provider.VirtualMachineConfigs())
}

func TestCPUPartitionConfigLoadedValidation(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: CPUPartitionConfig
virtualMachines:
    cpus: 4-7
    slices:
        - name: database
          cpus: 4-5
        - name: database
          cpus: 5-6
`))
	require.NoError(t, err)

	_, err = provider.ValidateAsClient(validationMode{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CPUPartitionConfig: ")
	assert.Contains(t, err.Error(), `virtualMachines.slices[1]: duplicate slice name "database"`)
	assert.Contains(t, err.Error(), `virtualMachines.slices[1]: cpus "5-6" overlap another slice on "5"`)
}

//nolint:maintidx
func TestCPUPartitionConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.CPUPartitionConfigV1Alpha1

		expectedError string
	}{
		{
			name: "empty",
			cfg:  runtime.NewCPUPartitionConfigV1Alpha1,

			expectedError: "at least one root must be bounded",
		},
		{
			name: "accepted",
			cfg:  acceptedCPUPartitionConfig,
		},
		{
			name: "single root",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: "2-3"}

				return cfg
			},
		},
		{
			name: "present root with an empty list",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.InitConfig = &runtime.CPUPartitionRoot{}
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{}

				return cfg
			},

			expectedError: "init.cpus is required\nvirtualMachines.cpus is required",
		},
		{
			name: "unparseable and out of range lists",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.SystemConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-1000"}
				cfg.PodRuntimeConfig = &runtime.CPUPartitionRoot{RootCPUs: "3-1"}
				cfg.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-3,^1"}
				cfg.TalosContainersConfig = &runtime.CPUPartitionRoot{RootCPUs: "a"}

				return cfg
			},

			expectedError: `system.cpus "0-1000": 1000 is out of range, IDs must be between 0 and 999` + "\n" +
				`podruntime.cpus "3-1": invalid range "3-1" (3 > 1)` + "\n" +
				`kubepods.cpus "0-3,^1": strconv.Atoi: parsing "^1": invalid syntax` + "\n" +
				`taloscontainers.cpus "a": strconv.Atoi: parsing "a": invalid syntax`,
		},
		{
			name: "overlapping roots are allowed without an exclusive slice",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.InitConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-3"}
				cfg.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: "2-7"}
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
					RootCPUs: "4-7",
					SlicesConfig: []runtime.CPUPartitionSlice{
						{SliceName: "batch", SliceCPUs: "6-7"},
					},
				}

				return cfg
			},
		},
		{
			name: "slice outside the virtual machine root",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
					RootCPUs: "4-7",
					SlicesConfig: []runtime.CPUPartitionSlice{
						{SliceName: "database", SliceCPUs: "7-8"},
					},
				}

				return cfg
			},

			expectedError: `virtualMachines.slices[0]: cpus "7-8" must be a subset of virtualMachines.cpus "4-7"`,
		},
		{
			name: "overlapping slices",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
					RootCPUs: "4-7",
					SlicesConfig: []runtime.CPUPartitionSlice{
						{SliceName: "database", SliceCPUs: "4-5"},
						{SliceName: "cache", SliceCPUs: "5-6"},
					},
				}

				return cfg
			},

			expectedError: `virtualMachines.slices[1]: cpus "5-6" overlap another slice on "5"`,
		},
		{
			name: "slices may cover the whole root",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
					RootCPUs: "4-7",
					SlicesConfig: []runtime.CPUPartitionSlice{
						{SliceName: "database", SliceCPUs: "4-5"},
						{SliceName: "cache", SliceCPUs: "6-7"},
					},
				}

				return cfg
			},
		},
		{
			name: "slice names",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
					RootCPUs: "0-9",
					SlicesConfig: []runtime.CPUPartitionSlice{
						{SliceName: "", SliceCPUs: "0"},
						{SliceName: "shared", SliceCPUs: "1"},
						{SliceName: "data base", SliceCPUs: "2"},
						{SliceName: strings.Repeat("a", 64), SliceCPUs: "3"},
						{SliceName: "database", SliceCPUs: "4"},
						{SliceName: "database", SliceCPUs: "5"},
						{SliceName: "empty", SliceCPUs: ""},
					},
				}

				return cfg
			},

			expectedError: "virtualMachines.slices[0]: name is required\n" +
				`virtualMachines.slices[1]: name "shared" is reserved` + "\n" +
				`virtualMachines.slices[2]: name "data base": name can only contain ASCII letters, digits and hyphens` + "\n" +
				`virtualMachines.slices[3]: name "` + strings.Repeat("a", 64) + `" must be 63 characters or fewer` + "\n" +
				`virtualMachines.slices[5]: duplicate slice name "database"` + "\n" +
				"virtualMachines.slices[6].cpus is required",
		},
		{
			name: "exclusive slice requires every other root",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.InitConfig = &runtime.CPUPartitionRoot{RootCPUs: "0-1"}
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
					RootCPUs: "4-7",
					SlicesConfig: []runtime.CPUPartitionSlice{
						{SliceName: "database", SliceCPUs: "6-7", SliceExclusive: new(true)},
					},
				}

				return cfg
			},

			expectedError: `virtualMachines.slices[0]: exclusive slice "database" requires system to be bounded` + "\n" +
				`virtualMachines.slices[0]: exclusive slice "database" requires podruntime to be bounded` + "\n" +
				`virtualMachines.slices[0]: exclusive slice "database" requires kubepods to be bounded` + "\n" +
				`virtualMachines.slices[0]: exclusive slice "database" requires taloscontainers to be bounded`,
		},
		{
			name: "exclusive slice overlapping other roots",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := acceptedCPUPartitionConfig()
				cfg.KubepodsConfig.RootCPUs = "2-5"
				cfg.TalosContainersConfig.RootCPUs = "0-1,4"

				return cfg
			},

			expectedError: `virtualMachines.slices[0]: exclusive slice "database" overlaps kubepods on "4-5"` + "\n" +
				`virtualMachines.slices[0]: exclusive slice "database" overlaps taloscontainers on "4"`,
		},
		{
			name: "a non-exclusive slice may overlap other roots",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := acceptedCPUPartitionConfig()
				cfg.KubepodsConfig.RootCPUs = "2-5"
				cfg.VirtualMachinesConfig.SlicesConfig[0].SliceExclusive = nil

				return cfg
			},
		},
		{
			name: "slice errors are reported once for an unparseable root",
			cfg: func() *runtime.CPUPartitionConfigV1Alpha1 {
				cfg := runtime.NewCPUPartitionConfigV1Alpha1()
				cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
					RootCPUs: "x",
					SlicesConfig: []runtime.CPUPartitionSlice{
						{SliceName: "database", SliceCPUs: "6-7"},
					},
				}

				return cfg
			},

			expectedError: `virtualMachines.cpus "x": strconv.Atoi: parsing "x": invalid syntax`,
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

func TestCPUPartitionConfigAccessors(t *testing.T) {
	t.Parallel()

	t.Run("accepted", func(t *testing.T) {
		t.Parallel()

		cfg := acceptedCPUPartitionConfig()

		assert.Equal(t, map[config.CPUPartitionRoot]string{
			constants.CgroupInit:                "0-1",
			constants.CgroupSystem:              "0-1",
			constants.CgroupPodRuntimeRoot:      "0-1",
			constants.CgroupKubepods:            "2-3",
			constants.CgroupTalosContainersRoot: "0-1",
			constants.CgroupVirtualMachinesRoot: "4-7",
		}, cfg.Roots())

		slices := cfg.Slices()
		require.Len(t, slices, 1)
		assert.Equal(t, "database", slices[0].Name())
		assert.Equal(t, "4-5", slices[0].CPUs())
		assert.True(t, slices[0].Exclusive())
	})

	t.Run("omitted roots are absent, lists are canonical", func(t *testing.T) {
		t.Parallel()

		cfg := runtime.NewCPUPartitionConfigV1Alpha1()
		cfg.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: "3,2,2"}
		cfg.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
			RootCPUs: "7,4-6",
			SlicesConfig: []runtime.CPUPartitionSlice{
				{SliceName: "batch", SliceCPUs: "+5,4"},
			},
		}

		assert.Equal(t, map[config.CPUPartitionRoot]string{
			constants.CgroupKubepods:            "2-3",
			constants.CgroupVirtualMachinesRoot: "4-7",
		}, cfg.Roots())

		slices := cfg.Slices()
		require.Len(t, slices, 1)
		assert.Equal(t, "4-5", slices[0].CPUs())
		assert.False(t, slices[0].Exclusive())
	})

	t.Run("no virtual machine root means no slices", func(t *testing.T) {
		t.Parallel()

		cfg := runtime.NewCPUPartitionConfigV1Alpha1()
		cfg.InitConfig = &runtime.CPUPartitionRoot{RootCPUs: "0"}

		assert.Equal(t, map[config.CPUPartitionRoot]string{constants.CgroupInit: "0"}, cfg.Roots())
		assert.Empty(t, cfg.Slices())
	})
}

func TestCPUPartitionConfigClone(t *testing.T) {
	t.Parallel()

	cfg := acceptedCPUPartitionConfig()

	clone, ok := cfg.Clone().(*runtime.CPUPartitionConfigV1Alpha1)
	require.True(t, ok)
	assert.Equal(t, cfg, clone)

	clone.KubepodsConfig.RootCPUs = "2"
	clone.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs = "4"
	*clone.VirtualMachinesConfig.SlicesConfig[0].SliceExclusive = false

	assert.Equal(t, "2-3", cfg.KubepodsConfig.RootCPUs)
	assert.Equal(t, "4-5", cfg.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs)
	assert.True(t, cfg.Slices()[0].Exclusive())
}

func TestCPUPartitionConfigMergeSlices(t *testing.T) {
	t.Parallel()

	base := acceptedCPUPartitionConfig()

	patch := runtime.NewCPUPartitionConfigV1Alpha1()
	patch.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{
		SlicesConfig: []runtime.CPUPartitionSlice{{SliceName: "cache", SliceCPUs: "6"}},
	}

	left, err := container.New(base)
	require.NoError(t, err)

	right, err := container.New(patch)
	require.NoError(t, err)

	merged, err := configpatcher.StrategicMerge(left, configpatcher.NewStrategicMergePatch(right))
	require.NoError(t, err)

	documents := merged.Documents()
	require.Len(t, documents, 1)

	cfg, ok := documents[0].(*runtime.CPUPartitionConfigV1Alpha1)
	require.True(t, ok)

	assert.Equal(t, "4-7", cfg.VirtualMachinesConfig.RootCPUs)
	assert.Equal(t, []runtime.CPUPartitionSlice{{SliceName: "cache", SliceCPUs: "6"}}, cfg.VirtualMachinesConfig.SlicesConfig)
	assert.Equal(t, "2-3", cfg.KubepodsConfig.RootCPUs)
}

func TestCPUPartitionConfigSingleton(t *testing.T) {
	t.Parallel()

	_, err := container.New(acceptedCPUPartitionConfig(), acceptedCPUPartitionConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate document: CPUPartitionConfig")
}
