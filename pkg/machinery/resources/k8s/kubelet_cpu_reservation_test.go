// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	k8sapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/k8s"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

func TestKubeletCPUReservationRoundTrip(t *testing.T) {
	t.Parallel()

	res := k8s.NewKubeletCPUReservation()
	assert.Equal(t, k8s.NamespaceName, res.Metadata().Namespace())
	assert.Equal(t, k8s.KubeletCPUReservationType, res.Metadata().Type())
	assert.Equal(t, k8s.KubeletID, res.Metadata().ID())
	assert.Equal(t, k8s.NamespaceName, res.ResourceDefinition().DefaultNamespace)

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "managed: false\n", string(marshaled))

	*res.TypedSpec() = k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0-1"}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec k8sapi.KubeletCPUReservationSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.True(t, spec.GetManaged())
	assert.Equal(t, "0-1", spec.GetReservedCpUs())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*k8s.KubeletCPUReservation).TypedSpec())

	clone := res.DeepCopy().(*k8s.KubeletCPUReservation)
	clone.TypedSpec().ReservedCPUs = "0"
	assert.Equal(t, "0-1", res.TypedSpec().ReservedCPUs)

	marshaled, err = yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, "managed: true\nreservedCPUs: 0-1\n", string(marshaled))

	var yamlSpec k8s.KubeletCPUReservationSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &yamlSpec))
	assert.Equal(t, *res.TypedSpec(), yamlSpec)
}
