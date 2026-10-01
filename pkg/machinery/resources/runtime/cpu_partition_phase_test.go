// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	enumsapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/enums"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestCPUPartitionPhase(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		phase     runtime.CPUPartitionPhase
		text      string
		wire      enumsapi.RuntimeCPUPartitionPhase
		admission bool
	}{
		{runtime.CPUPartitionPhaseReady, "ready", enumsapi.RuntimeCPUPartitionPhase_CPU_PARTITION_PHASE_READY, true},
		{runtime.CPUPartitionPhaseConverging, "converging", enumsapi.RuntimeCPUPartitionPhase_CPU_PARTITION_PHASE_CONVERGING, false},
		{runtime.CPUPartitionPhaseBlocked, "blocked", enumsapi.RuntimeCPUPartitionPhase_CPU_PARTITION_PHASE_BLOCKED, true},
		{runtime.CPUPartitionPhaseRestoring, "restoring", enumsapi.RuntimeCPUPartitionPhase_CPU_PARTITION_PHASE_RESTORING, false},
		{runtime.CPUPartitionPhaseApplying, "applying", enumsapi.RuntimeCPUPartitionPhase_CPU_PARTITION_PHASE_APPLYING, false},
	} {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.text, test.phase.String())
			assert.Equal(t, int32(test.wire), int32(test.phase), "Go and protobuf enum values must agree")
			assert.Equal(t, test.admission, test.phase.AdmissionOpen())

			parsed, err := runtime.CPUPartitionPhaseString(test.text)
			require.NoError(t, err)
			assert.Equal(t, test.phase, parsed)
		})
	}

	assert.Len(t, runtime.CPUPartitionPhaseValues(), len(enumsapi.RuntimeCPUPartitionPhase_name))
}

func TestCPUPartitionStatusZero(t *testing.T) {
	t.Parallel()

	res := runtime.NewCPUPartitionStatus()
	assert.Equal(t, runtime.CPUPartitionStatusID, res.Metadata().ID())
	assert.Equal(t, runtime.CPUPartitionSpecID, res.Metadata().ID(), "status and spec are singletons sharing an ID")
	assert.Equal(t, runtime.NamespaceName, res.ResourceDefinition().DefaultNamespace)

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "phase: ready\n", string(marshaled))

	_, ok := res.TypedSpec().Target("init")
	assert.False(t, ok)

	res.TypedSpec().DeleteTarget("init")
	assert.Empty(t, res.TypedSpec().Targets)

	res.TypedSpec().SetTarget(runtime.CPUPartitionTargetStatus{Key: "kubepods", LastApplied: "2-3"})
	res.TypedSpec().SetTarget(runtime.CPUPartitionTargetStatus{Key: "kubepods", LastApplied: "2"})
	require.Len(t, res.TypedSpec().Targets, 1, "SetTarget replaces an existing key")
	assert.Equal(t, "2", res.TypedSpec().Targets[0].LastApplied)

	clone := res.DeepCopy().(*runtime.CPUPartitionStatus)
	clone.TypedSpec().Targets[0].LastApplied = "3"
	assert.Equal(t, "2", res.TypedSpec().Targets[0].LastApplied)
}
