// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hypervisorapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestContentLibraryStatusRoundTrip(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	*res.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID:    "IMAGES",
		Path:        "/var/mnt/images",
		Ready:       true,
		Error:       "",
		Fingerprint: "d41d8cd98f00b204",
	}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.ContentLibraryStatusSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, res.TypedSpec().VolumeID, spec.GetVolumeId())
	assert.Equal(t, res.TypedSpec().Path, spec.GetPath())
	assert.Equal(t, res.TypedSpec().Ready, spec.GetReady())
	assert.Equal(t, res.TypedSpec().Fingerprint, spec.GetFingerprint())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*hypervisor.ContentLibraryStatus).TypedSpec())

	clone := res.DeepCopy().(*hypervisor.ContentLibraryStatus)
	clone.TypedSpec().Path = "changed"
	assert.NotEqual(t, clone.TypedSpec().Path, res.TypedSpec().Path)
	assert.Equal(t, hypervisor.NamespaceName, res.ResourceDefinition().DefaultNamespace)
}
