// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	runtimeapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/runtime"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestCPUPartitionSpecRoundTrip(t *testing.T) {
	t.Parallel()

	res := runtime.NewCPUPartitionSpec()
	*res.TypedSpec() = runtime.CPUPartitionSpecSpec{
		Enabled: true,
		Roots: map[string]string{
			"init":            "0-1",
			"kubepods":        "2-3",
			"virtualMachines": "4-7",
		},
		Slices: []runtime.CPUPartitionSliceSpec{
			{Name: "database", CPUs: "4-5", Exclusive: true},
			{Name: "batch", CPUs: "6"},
		},
	}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec runtimeapi.CPUPartitionSpecSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.True(t, spec.GetEnabled())
	assert.Equal(t, res.TypedSpec().Roots, spec.GetRoots())
	require.Len(t, spec.GetSlices(), 2)
	assert.Equal(t, "database", spec.GetSlices()[0].GetName())
	assert.Equal(t, "4-5", spec.GetSlices()[0].GetCpUs())
	assert.True(t, spec.GetSlices()[0].GetExclusive())
	assert.False(t, spec.GetSlices()[1].GetExclusive())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*runtime.CPUPartitionSpec).TypedSpec())

	clone := res.DeepCopy().(*runtime.CPUPartitionSpec)
	clone.TypedSpec().Roots["kubepods"] = "2"
	clone.TypedSpec().Slices[0].CPUs = "4"
	assert.Equal(t, "2-3", res.TypedSpec().Roots["kubepods"])
	assert.Equal(t, "4-5", res.TypedSpec().Slices[0].CPUs)
	assert.Equal(t, runtime.NamespaceName, res.ResourceDefinition().DefaultNamespace)
	assert.Equal(t, runtime.CPUPartitionSpecID, res.Metadata().ID())

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, `enabled: true
roots:
    init: 0-1
    kubepods: 2-3
    virtualMachines: 4-7
slices:
    - name: database
      cpus: 4-5
      exclusive: true
    - name: batch
      cpus: "6"
      exclusive: false
`, string(marshaled))

	var yamlSpec runtime.CPUPartitionSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &yamlSpec))
	assert.Equal(t, *res.TypedSpec(), yamlSpec)

	disabled := runtime.NewCPUPartitionSpec()
	marshaled, err = yaml.Marshal(disabled.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "enabled: false\n", string(marshaled))
}

func TestCPUPartitionStatusRoundTrip(t *testing.T) {
	t.Parallel()

	res := runtime.NewCPUPartitionStatus()
	*res.TypedSpec() = runtime.CPUPartitionStatusSpec{
		Phase:     runtime.CPUPartitionPhaseBlocked,
		Exclusive: []string{"database"},
		Blocked:   []runtime.CPUPartitionBlock{{Reason: "swap", VirtualMachines: []string{"db", "web"}, CPUs: "4-7"}},
	}
	res.TypedSpec().SetTarget(runtime.CPUPartitionTargetStatus{Key: "virtualMachines/shared", Initial: "", LastApplied: "6-7", Intended: "6-7"})
	res.TypedSpec().SetTarget(runtime.CPUPartitionTargetStatus{Key: "init", Initial: "0-7", LastApplied: "0-1", Intended: "0-1"})

	require.Equal(t, "init", res.TypedSpec().Targets[0].Key, "targets are kept sorted")

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec runtimeapi.CPUPartitionStatusSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, int32(runtime.CPUPartitionPhaseBlocked), int32(spec.GetPhase()))
	require.Len(t, spec.GetTargets(), 2)
	assert.Equal(t, "0-1", spec.GetTargets()[0].GetLastApplied())
	assert.Equal(t, []string{"db", "web"}, spec.GetBlocked()[0].GetVirtualMachines())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*runtime.CPUPartitionStatus).TypedSpec())

	target, ok := res.TypedSpec().Target("init")
	require.True(t, ok)
	assert.Equal(t, "0-7", target.Initial)

	res.TypedSpec().DeleteTarget("init")
	_, ok = res.TypedSpec().Target("init")
	assert.False(t, ok)

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Contains(t, string(marshaled), "phase: blocked\n")
}
