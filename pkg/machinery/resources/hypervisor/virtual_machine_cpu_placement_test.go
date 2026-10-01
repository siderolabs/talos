// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	hypervisorapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineCPUPlacementRegister(t *testing.T) {
	t.Parallel()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, registry.NewResourceRegistry(resources).Register(t.Context(), &hypervisor.VirtualMachineCPUPlacement{}))
}

func TestVirtualMachineCPUPlacementRoundTrip(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, "db")
	assert.Equal(t, hypervisor.VirtualMachineCPUPlacementType, res.Metadata().Type())
	assert.Equal(t, hypervisor.NamespaceName, res.ResourceDefinition().DefaultNamespace)

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "partition: \"\"\nexclusive: false\n", string(marshaled))

	*res.TypedSpec() = hypervisor.VirtualMachineCPUPlacementSpec{
		Partition: "/virtualmachines.partition/database.partition",
		Slice:     "database",
		Exclusive: true,
	}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.VirtualMachineCPUPlacementSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, "/virtualmachines.partition/database.partition", spec.GetPartition())
	assert.Equal(t, "database", spec.GetSlice())
	assert.True(t, spec.GetExclusive())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*hypervisor.VirtualMachineCPUPlacement).TypedSpec())

	clone := res.DeepCopy().(*hypervisor.VirtualMachineCPUPlacement)
	clone.TypedSpec().Partition = "/virtualmachines.partition/shared.partition"
	assert.Equal(t, "/virtualmachines.partition/database.partition", res.TypedSpec().Partition)

	marshaled, err = yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "partition: /virtualmachines.partition/database.partition\nslice: database\nexclusive: true\n", string(marshaled))

	var yamlSpec hypervisor.VirtualMachineCPUPlacementSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &yamlSpec))
	assert.Equal(t, *res.TypedSpec(), yamlSpec)
}

func TestVirtualMachineCPUSpecSliceDefault(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest")
	res.TypedSpec().CPU = hypervisor.VirtualMachineCPUSpec{Count: 2}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.VirtualMachineSpecSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Empty(t, spec.GetCpu().GetSlice())

	marshaled, err := yaml.Marshal(res.TypedSpec().CPU)
	require.NoError(t, err)
	assert.Equal(t, "count: 2\n", string(marshaled))
}
