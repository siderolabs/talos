// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"math"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	runtimeapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/runtime"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestWorkloadMemorySpecRoundTrip(t *testing.T) {
	t.Parallel()

	res := runtime.NewWorkloadMemorySpec()
	assert.Equal(t, runtime.NamespaceName, res.Metadata().Namespace())
	assert.Equal(t, runtime.WorkloadMemorySpecType, res.Metadata().Type())
	assert.Equal(t, runtime.WorkloadMemorySpecID, res.Metadata().ID())
	assert.Equal(t, runtime.NamespaceName, res.ResourceDefinition().DefaultNamespace)

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "{}\n", string(marshaled))

	*res.TypedSpec() = runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 4 << 30,
		VirtualMachinesLimit: math.MaxInt64,
	}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec runtimeapi.WorkloadMemorySpecSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, uint64(4<<30), spec.GetTalosContainersLimit())
	assert.Equal(t, uint64(math.MaxInt64), spec.GetVirtualMachinesLimit())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*runtime.WorkloadMemorySpec).TypedSpec())

	clone := res.DeepCopy().(*runtime.WorkloadMemorySpec)
	clone.TypedSpec().TalosContainersLimit = 1
	assert.Equal(t, uint64(4<<30), res.TypedSpec().TalosContainersLimit)

	marshaled, err = yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "talosContainersLimit: 4294967296\nvirtualMachinesLimit: 9223372036854775807\n", string(marshaled))

	var yamlSpec runtime.WorkloadMemorySpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &yamlSpec))
	assert.Equal(t, *res.TypedSpec(), yamlSpec)

	partial := runtime.NewWorkloadMemorySpec()
	partial.TypedSpec().VirtualMachinesLimit = 32 << 30

	marshaled, err = yaml.Marshal(partial.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "virtualMachinesLimit: 34359738368\n", string(marshaled))

	encoded, err = protobuf.FromResource(partial)
	require.NoError(t, err)
	wire, err = encoded.Marshal()
	require.NoError(t, err)

	decoded, err = protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err = protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	assert.Equal(t, partial.TypedSpec(), roundTrip.(*runtime.WorkloadMemorySpec).TypedSpec())
	assert.Zero(t, roundTrip.(*runtime.WorkloadMemorySpec).TypedSpec().TalosContainersLimit)
}
