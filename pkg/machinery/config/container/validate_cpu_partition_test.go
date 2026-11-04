// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
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
		{name: "booted policy changed", boot: &bootPolicy, doc: newCPUPartitionDocument("1-2", database, batch), warnings: []string{rebootWarning}},
		{name: "booted policy removed", boot: &bootPolicy, warnings: []string{rebootWarning}},
		{name: "policy added", boot: &runtimeres.CPUPartitionSpecSpec{}, doc: newCPUPartitionDocument("1-3"), warnings: []string{rebootWarning}},
		{name: "no policy", boot: &runtimeres.CPUPartitionSpecSpec{}},
		{
			name:          "kubepods takes every online CPU",
			doc:           newCPUPartitionDocument("0-7"),
			expectedError: `kubepods.cpus "0-7" leaves no online CPU outside kubepods`,
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
