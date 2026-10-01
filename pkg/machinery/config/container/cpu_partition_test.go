// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"net/url"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// newCPUPartitionDoc is the accepted partition: every root bounded, `database` exclusive on 4-5,
// leaving 6-7 as the shared remainder of the virtual machine root.
func newCPUPartitionDoc() *runtimecfg.CPUPartitionConfigV1Alpha1 {
	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.SystemConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.PodRuntimeConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "2-3"}
	doc.TalosContainersConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.VirtualMachinesConfig = &runtimecfg.CPUPartitionVirtualMachines{
		RootCPUs: "4-7",
		SlicesConfig: []runtimecfg.CPUPartitionSlice{
			{SliceName: "database", SliceCPUs: "4-5", SliceExclusive: new(true)},
		},
	}

	return doc
}

// newSlicedVirtualMachineDoc builds a valid VirtualMachineConfig selecting the given slice.
func newSlicedVirtualMachineDoc(name, slice string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI
	doc.CPUConfig.CPUCount = 2
	doc.CPUConfig.CPUSlice = slice
	doc.MemoryConfig.MemorySize = meta.MustByteSize("1GiB")

	return doc
}

func pinVirtualMachine(doc *hypervisorcfg.VirtualMachineConfigV1Alpha1, emulator string, vcpus ...string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc.CPUConfig.TopologyConfig.PinningConfig.EmulatorConfig = emulator

	for i, cpus := range vcpus {
		doc.CPUConfig.TopologyConfig.PinningConfig.VCPUsConfig = append(doc.CPUConfig.TopologyConfig.PinningConfig.VCPUsConfig,
			hypervisorcfg.VirtualMachineVCPUPin{PinVCPU: uint32(i), PinCPUs: cpus})
	}

	return doc
}

func newKubeletDoc(extraConfig map[string]any) *k8s.KubeletConfigV1Alpha1 {
	doc := k8s.NewKubeletConfigV1Alpha1()
	doc.KubeletImage = constants.KubeletImage + ":v" + constants.DefaultKubernetesVersion
	doc.KubeletConfig.Object = extraConfig

	return doc
}

func newLegacyKubeletConfig(t *testing.T, extraConfig map[string]any) *v1alpha1.Config {
	t.Helper()

	return &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{ //nolint:staticcheck // testing legacy features
				Endpoint: &v1alpha1.Endpoint{
					URL: must.Value(url.Parse("https://localhost:6443"))(t),
				},
			},
		},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineCA: &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
			},
			MachineKubelet: &v1alpha1.KubeletConfig{ //nolint:staticcheck // testing legacy features
				KubeletExtraConfig: meta.Unstructured{Object: extraConfig}, //nolint:staticcheck // testing legacy features
			},
		},
	}
}

//nolint:maintidx
func TestCPUPartitionCrossDocumentValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		docs []config.Document

		expectedErrs   []string
		unexpectedErrs []string
	}{
		{
			name: "no document, no slice: count-only and pin-only virtual machines are untouched",
			docs: []config.Document{
				newSlicedVirtualMachineDoc("vm1", ""),
				pinVirtualMachine(newSlicedVirtualMachineDoc("vm2", ""), "0-1", "8", "9-10"),
			},
		},
		{
			name: "slice without a document",
			docs: []config.Document{
				newSlicedVirtualMachineDoc("vm1", "database"),
			},
			expectedErrs: []string{
				`virtual machine "vm1": cpu.slice "database": no CPUPartitionConfig declares any slice`,
			},
		},
		{
			name: "accepted: exclusive owner with fitting pins and a shared virtual machine",
			docs: []config.Document{
				newCPUPartitionDoc(),
				pinVirtualMachine(newSlicedVirtualMachineDoc("vm1", "database"), "4", "4", "5"),
				pinVirtualMachine(newSlicedVirtualMachineDoc("vm2", ""), "6-7", "7"),
			},
		},
		{
			name: "undeclared slice",
			docs: []config.Document{
				newCPUPartitionDoc(),
				newSlicedVirtualMachineDoc("vm1", "cache"),
			},
			expectedErrs: []string{
				`virtual machine "vm1": cpu.slice "cache": no CPUPartitionConfig declares slice "cache"`,
			},
		},
		{
			name: "two owners of an exclusive slice, one of them stopped",
			docs: func() []config.Document {
				stopped := newSlicedVirtualMachineDoc("vm2", "database")
				stopped.PowerStateConfig = hypervisorhelpers.PowerStateStopped

				return []config.Document{
					newCPUPartitionDoc(),
					newSlicedVirtualMachineDoc("vm1", "database"),
					stopped,
				}
			}(),
			expectedErrs: []string{
				`exclusive slice "database" is selected by more than one virtual machine: [vm1 vm2]`,
			},
		},
		{
			name: "a non-exclusive slice takes several owners",
			docs: func() []config.Document {
				partition := newCPUPartitionDoc()
				partition.VirtualMachinesConfig.SlicesConfig[0].SliceExclusive = nil

				return []config.Document{
					partition,
					newSlicedVirtualMachineDoc("vm1", "database"),
					newSlicedVirtualMachineDoc("vm2", "database"),
				}
			}(),
		},
		{
			name: "pins outside the slice",
			docs: []config.Document{
				newCPUPartitionDoc(),
				pinVirtualMachine(newSlicedVirtualMachineDoc("vm1", "database"), "0", "4", "5-6"),
			},
			expectedErrs: []string{
				`virtual machine "vm1": cpu.topology.pinning.vcpus[1].cpus "5-6" must be a subset of slice "database" ("4-5")`,
				`virtual machine "vm1": cpu.topology.pinning.emulator "0" must be a subset of slice "database" ("4-5")`,
			},
			unexpectedErrs: []string{"vcpus[0]"},
		},
		{
			name: "pins outside the shared remainder, including on a named slice",
			docs: []config.Document{
				newCPUPartitionDoc(),
				pinVirtualMachine(newSlicedVirtualMachineDoc("vm1", ""), "", "4", "6"),
			},
			expectedErrs: []string{
				`virtual machine "vm1": cpu.topology.pinning.vcpus[0].cpus "4" must be a subset of the shared remainder "6-7" of virtualMachines.cpus`,
			},
			unexpectedErrs: []string{"vcpus[1]"},
		},
		{
			name: "slices leave no remainder for a slice-less virtual machine",
			docs: func() []config.Document {
				partition := newCPUPartitionDoc()
				partition.VirtualMachinesConfig.SlicesConfig = append(partition.VirtualMachinesConfig.SlicesConfig,
					runtimecfg.CPUPartitionSlice{SliceName: "cache", SliceCPUs: "6-7"})

				return []config.Document{
					partition,
					newSlicedVirtualMachineDoc("vm1", "database"),
					newSlicedVirtualMachineDoc("vm2", ""),
				}
			}(),
			expectedErrs: []string{
				`virtual machine "vm2": no cpu.slice selected but the named slices leave no CPU of virtualMachines.cpus "4-7"`,
			},
			unexpectedErrs: []string{`virtual machine "vm1"`},
		},
		{
			name: "an empty remainder is fine while every virtual machine selects a slice",
			docs: func() []config.Document {
				partition := newCPUPartitionDoc()
				partition.VirtualMachinesConfig.SlicesConfig = append(partition.VirtualMachinesConfig.SlicesConfig,
					runtimecfg.CPUPartitionSlice{SliceName: "cache", SliceCPUs: "6-7"})

				return []config.Document{
					partition,
					newSlicedVirtualMachineDoc("vm1", "database"),
					newSlicedVirtualMachineDoc("vm2", "cache"),
				}
			}(),
		},
		{
			name: "no virtual machine root leaves slice-less virtual machines unconstrained",
			docs: func() []config.Document {
				partition := runtimecfg.NewCPUPartitionConfigV1Alpha1()
				partition.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "2-3"}

				return []config.Document{
					partition,
					pinVirtualMachine(newSlicedVirtualMachineDoc("vm1", ""), "0-1", "8"),
				}
			}(),
		},
		{
			name: "explicit reservedSystemCPUs in KubeletConfig while kubepods is bounded, even if equal",
			docs: []config.Document{
				newCPUPartitionDoc(),
				newKubeletDoc(map[string]any{"reservedSystemCPUs": "0-1,4-7"}),
			},
			expectedErrs: []string{
				`kubelet configuration field "reservedSystemCPUs" is owned by CPUPartitionConfig while kubepods is bounded`,
			},
		},
		{
			name: "other kubelet reservations are preserved",
			docs: []config.Document{
				newCPUPartitionDoc(),
				newKubeletDoc(map[string]any{
					"systemReserved":   map[string]any{"cpu": "500m"},
					"kubeReserved":     map[string]any{"cpu": "500m"},
					"cpuManagerPolicy": "static",
				}),
			},
		},
		{
			name: "reservedSystemCPUs is the user's while kubepods is unbounded",
			docs: func() []config.Document {
				partition := runtimecfg.NewCPUPartitionConfigV1Alpha1()
				partition.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}

				return []config.Document{
					partition,
					newKubeletDoc(map[string]any{"reservedSystemCPUs": "0-1"}),
				}
			}(),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(test.docs...)
			require.NoError(t, err)

			_, err = ctr.ValidateAsClient(validationMode{})

			if len(test.expectedErrs) == 0 {
				assert.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, expectedErr := range test.expectedErrs {
				assert.ErrorContains(t, err, expectedErr)
			}

			for _, unexpectedErr := range test.unexpectedErrs {
				assert.NotContains(t, err.Error(), unexpectedErr)
			}
		})
	}
}

// The legacy .machine.kubelet.extraConfig is the other place reservedSystemCPUs can be set.
func TestCPUPartitionLegacyKubeletConflict(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		extraConfig map[string]any
		partition   *runtimecfg.CPUPartitionConfigV1Alpha1

		expectedErr string
	}{
		{
			name:        "reserved while kubepods is bounded",
			extraConfig: map[string]any{"reservedSystemCPUs": "0-1"},
			partition:   newCPUPartitionDoc(),
			expectedErr: `kubelet configuration field "reservedSystemCPUs" is owned by CPUPartitionConfig while kubepods is bounded`,
		},
		{
			name:        "other fields are preserved",
			extraConfig: map[string]any{"systemReserved": map[string]any{"cpu": "500m"}},
			partition:   newCPUPartitionDoc(),
		},
		{
			name:        "reserved while kubepods is unbounded",
			extraConfig: map[string]any{"reservedSystemCPUs": "0-1"},
			partition: func() *runtimecfg.CPUPartitionConfigV1Alpha1 {
				partition := runtimecfg.NewCPUPartitionConfigV1Alpha1()
				partition.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}

				return partition
			}(),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(newLegacyKubeletConfig(t, test.extraConfig), test.partition)
			require.NoError(t, err)

			_, err = ctr.ValidateAsClient(validationMode{})

			if test.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedErr)
			}
		})
	}
}

// Container mode still validates the documents structurally; the runtime no-op lives in the controllers.
func TestCPUPartitionValidatesInContainerMode(t *testing.T) {
	t.Parallel()

	ctr, err := container.New(newCPUPartitionDoc(), newSlicedVirtualMachineDoc("vm1", "cache"))
	require.NoError(t, err)

	_, err = ctr.ValidateAsClient(validationMode{inContainer: true})
	require.Error(t, err)
	assert.ErrorContains(t, err, `no CPUPartitionConfig declares slice "cache"`)
}

func TestCPUPartitionConfigAccessor(t *testing.T) {
	t.Parallel()

	ctr, err := container.New(newSlicedVirtualMachineDoc("vm1", ""))
	require.NoError(t, err)
	assert.Nil(t, ctr.CPUPartitionConfig())

	ctr, err = container.New(newCPUPartitionDoc())
	require.NoError(t, err)
	require.NotNil(t, ctr.CPUPartitionConfig())
	assert.Equal(t, "2-3", ctr.CPUPartitionConfig().Roots()[config.CPUPartitionRootKubepods])
}
