// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

func TestNUMATopologyRoundTripAndCopy(t *testing.T) {
	original := hardware.NewNUMATopology()
	*original.TypedSpec() = hardware.NUMATopologySpec{
		Nodes:       []hardware.NUMANodeSpec{{ID: 7, CPUs: []uint32{3}, MemoryTotalBytes: 1024}},
		PresentCPUs: []uint32{3}, OnlineCPUs: []uint32{3},
	}

	encoded, err := protobuf.FromResource(original)
	require.NoError(t, err)

	wire, err := encoded.Marshal()
	require.NoError(t, err)

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)

	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	assert.Equal(t, original.TypedSpec(), roundTrip.(*hardware.NUMATopology).TypedSpec())

	clone := original.DeepCopy().(*hardware.NUMATopology)
	clone.TypedSpec().Nodes[0].CPUs[0] = 8
	clone.TypedSpec().Nodes[0].MemoryTotalBytes = 2048
	clone.TypedSpec().PresentCPUs[0] = 8
	clone.TypedSpec().OnlineCPUs[0] = 8
	assert.Equal(t, hardware.NUMATopologySpec{
		Nodes:       []hardware.NUMANodeSpec{{ID: 7, CPUs: []uint32{3}, MemoryTotalBytes: 1024}},
		PresentCPUs: []uint32{3}, OnlineCPUs: []uint32{3},
	}, *original.TypedSpec())
}

func TestRegisterResource(t *testing.T) {
	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []meta.ResourceWithRD{
		&hardware.BMCDevice{},
		&hardware.CPUCore{},
		&hardware.MemoryModule{},
		&hardware.NUMATopology{},
		&hardware.PCIDevice{},
		&hardware.PCIDriverRebindConfig{},
		&hardware.PCIDriverRebindStatus{},
		&hardware.PCRStatus{},
		&hardware.Processor{},
		&hardware.SystemInformation{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}
