// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func newCPUPartitionDocument(kubepods string, slices ...runtime.CPUPartitionSlice) *runtime.CPUPartitionConfigV1Alpha1 {
	doc := runtime.NewCPUPartitionConfigV1Alpha1()
	doc.SystemConfig = &runtime.CPUPartitionRoot{RootCPUs: "0"}
	doc.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: kubepods}

	if len(slices) > 0 {
		doc.VirtualMachinesConfig = &runtime.CPUPartitionVirtualMachines{RootCPUs: "4-7", SlicesConfig: slices}
	}

	return doc
}

func TestRuntimeValidateCPUPartition(t *testing.T) {
	t.Parallel()

	const rebootWarning = "CPUPartitionConfig: the CPU partition policy changes take effect only after a reboot"

	database := runtime.CPUPartitionSlice{SliceName: "database", SliceCPUs: "6-7"}
	batch := runtime.CPUPartitionSlice{SliceName: "batch", SliceCPUs: "5"}
	bootPolicy := container.ProjectCPUPartition(newCPUPartitionDocument("1-3", database, batch))

	for _, test := range []struct {
		name string
		boot *runtimeres.CPUPartitionSpecSpec
		doc  config.Document

		warnings      []string
		expectedError string
	}{
		{name: "nothing booted yet", doc: newCPUPartitionDocument("1-3")},
		{name: "booted policy unchanged", boot: &bootPolicy, doc: newCPUPartitionDocument("1-3", database, batch)},
		{name: "slices reordered", boot: &bootPolicy, doc: newCPUPartitionDocument("1-3", batch, database)},
		{
			name: "CPU lists respelled",
			boot: &bootPolicy,
			doc:  newCPUPartitionDocument("1,2,3", runtime.CPUPartitionSlice{SliceName: "database", SliceCPUs: "6,7"}, batch),
		},
		{name: "booted policy changed", boot: &bootPolicy, doc: newCPUPartitionDocument("1-2", database, batch), warnings: []string{rebootWarning}},
		{name: "booted policy removed", boot: &bootPolicy, warnings: []string{rebootWarning}},
		{name: "policy added", boot: &runtimeres.CPUPartitionSpecSpec{}, doc: newCPUPartitionDocument("1-3"), warnings: []string{rebootWarning}},
		{name: "no policy", boot: &runtimeres.CPUPartitionSpecSpec{}},
		{
			name:          "kubepods takes every online CPU",
			doc:           newCPUPartitionDocument("0-7"),
			expectedError: `kubepods.cpus "0-7" leaves no online CPU outside kubepods`,
		},
		{
			name:          "root names offline CPUs",
			doc:           newCPUPartitionDocument("1-15"),
			expectedError: `root kubepods names offline CPUs "8-15"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := state.WrapCore(inmem.NewState())

			topology := hardware.NewNUMATopology()
			topology.TypedSpec().OnlineCPUs = []uint32{0, 1, 2, 3, 4, 5, 6, 7}
			require.NoError(t, st.Create(ctx, topology))

			if test.boot != nil {
				spec := runtimeres.NewCPUPartitionSpec()
				*spec.TypedSpec() = *test.boot
				require.NoError(t, st.Create(ctx, spec))
			}

			var docs []config.Document

			if test.doc != nil {
				docs = append(docs, test.doc)
			}

			cfg, err := container.New(docs...)
			require.NoError(t, err)

			warnings, err := cfg.RuntimeValidate(ctx, st, validationMode{})

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.warnings, warnings)
		})
	}
}

func TestRuntimeValidateCPUPartitionBootKubelet(t *testing.T) {
	t.Parallel()

	newV1Alpha1 := func(extraConfig map[string]any) *v1alpha1.Config { //nolint:staticcheck // testing legacy features
		return &v1alpha1.Config{
			MachineConfig: &v1alpha1.MachineConfig{
				MachineType: "worker",
				MachineKubelet: &v1alpha1.KubeletConfig{ //nolint:staticcheck // testing legacy features
					KubeletImage:       "kubelet",                              //nolint:staticcheck // testing legacy features
					KubeletExtraConfig: meta.Unstructured{Object: extraConfig}, //nolint:staticcheck // testing legacy features
				},
			},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{ //nolint:staticcheck // testing legacy features
					Endpoint: &v1alpha1.Endpoint{URL: must.Value(url.Parse("https://localhost:6443"))(t)},
				},
				ClusterNetwork: &v1alpha1.ClusterNetworkConfig{}, //nolint:staticcheck // testing legacy features
			},
		}
	}

	const (
		rebootWarning   = "CPUPartitionConfig: the CPU partition policy changes take effect only after a reboot"
		conflictWarning = "CPUPartitionConfig: the kubelet configuration conflicts with the CPU partition policy of the boot and is not updated until a reboot: " +
			`"cpuManagerPolicy" none conflicts with the CPU partition kubepods root, which requires "static"`
	)

	for _, test := range []struct {
		name string
		boot runtimeres.CPUPartitionSpecSpec
		docs []config.Document

		warnings []string
	}{
		{
			name:     "booted kubepods bound, kubelet conflicts",
			boot:     container.ProjectCPUPartition(newCPUPartitionDocument("1-3")),
			docs:     []config.Document{newV1Alpha1(map[string]any{"cpuManagerPolicy": "none"})},
			warnings: []string{rebootWarning, conflictWarning},
		},
		{
			name:     "booted kubepods bound, kubelet compatible",
			boot:     container.ProjectCPUPartition(newCPUPartitionDocument("1-3")),
			docs:     []config.Document{newV1Alpha1(nil)},
			warnings: []string{rebootWarning},
		},
		{
			name:     "booted kubepods unbounded",
			boot:     runtimeres.CPUPartitionSpecSpec{},
			docs:     []config.Document{newV1Alpha1(map[string]any{"cpuManagerPolicy": "none"}), newCPUPartitionDocument("")},
			warnings: []string{rebootWarning},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := state.WrapCore(inmem.NewState())

			topology := hardware.NewNUMATopology()
			topology.TypedSpec().OnlineCPUs = []uint32{0, 1, 2, 3, 4, 5, 6, 7}
			require.NoError(t, st.Create(ctx, topology))

			spec := runtimeres.NewCPUPartitionSpec()
			*spec.TypedSpec() = test.boot
			require.NoError(t, st.Create(ctx, spec))

			cfg, err := container.New(test.docs...)
			require.NoError(t, err)

			warnings, err := cfg.RuntimeValidate(ctx, st, validationMode{})
			require.NoError(t, err)
			assert.Equal(t, test.warnings, warnings)
		})
	}
}

func TestValidateVirtualMachineCPUSlice(t *testing.T) {
	t.Parallel()

	vm := func(slice string) config.Document {
		doc := newBlankDiskVirtualMachineDoc("vm1")
		doc.CPUConfig.CPUSlice = slice

		return doc
	}

	database := runtime.CPUPartitionSlice{SliceName: "database", SliceCPUs: "6-7"}
	batch := runtime.CPUPartitionSlice{SliceName: "batch", SliceCPUs: "4-5"}

	for _, test := range []struct {
		name string
		docs []config.Document
		err  string
	}{
		{name: "declared slice", docs: []config.Document{newCPUPartitionDocument("1-3", database), vm("database")}},
		{name: "shared remainder", docs: []config.Document{newCPUPartitionDocument("1-3", database), vm("")}},
		{name: "no policy", docs: []config.Document{vm("database")}},
		{name: "virtual machine root unbounded", docs: []config.Document{newCPUPartitionDocument("1-3"), vm("database")}},
		{
			name: "undeclared slice",
			docs: []config.Document{newCPUPartitionDocument("1-3", database), vm("databse")},
			err:  `virtual machine "vm1": cpu.slice: CPUPartitionConfig declares no slice "databse"`,
		},
		{
			name: "no shared remainder",
			docs: []config.Document{newCPUPartitionDocument("1-3", database, batch), vm("")},
			err:  `virtual machine "vm1": cpu.slice: the CPUPartitionConfig slices take every CPU of the virtual machine root`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := container.New(append(poolDocs(), test.docs...)...)
			require.NoError(t, err)

			_, err = cfg.ValidateAsClient(validationMode{})

			if test.err == "" {
				require.NoError(t, err)

				return
			}

			assert.ErrorContains(t, err, test.err)
		})
	}
}
