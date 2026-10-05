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

func TestKubeletConfigKubepodsMemoryLimitRoundTrip(t *testing.T) {
	t.Parallel()

	res := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	res.TypedSpec().Image = "kubelet:v1.0.0"
	res.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	res.TypedSpec().ClusterDomain = "cluster.local"

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.NotContains(t, string(marshaled), "kubepodsMemoryLimit")

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec k8sapi.KubeletConfigSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Zero(t, spec.GetKubepodsMemoryLimit())

	res.TypedSpec().KubepodsMemoryLimit = 16 << 30

	encoded, err = protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err = encoded.Marshal()
	require.NoError(t, err)

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, uint64(16<<30), spec.GetKubepodsMemoryLimit())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*k8s.KubeletConfig).TypedSpec())

	clone := res.DeepCopy().(*k8s.KubeletConfig)
	clone.TypedSpec().KubepodsMemoryLimit = 1
	assert.Equal(t, uint64(16<<30), res.TypedSpec().KubepodsMemoryLimit)

	marshaled, err = yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Contains(t, string(marshaled), "kubepodsMemoryLimit: 17179869184\n")

	var yamlSpec k8s.KubeletConfigSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &yamlSpec))
	assert.Equal(t, *res.TypedSpec(), yamlSpec)
}
