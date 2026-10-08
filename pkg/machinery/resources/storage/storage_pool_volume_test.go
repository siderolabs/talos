// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storageapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/storage"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

func TestStoragePoolVolumeID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pool1/vm1__data.qcow2", storage.StoragePoolVolumeID("pool1", "vm1__data.qcow2"))
}

func TestStoragePoolVolumeSpecRoundTrip(t *testing.T) {
	t.Parallel()

	res := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", "vm1__data.qcow2"))
	*res.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool:          "pool1",
		Name:          "vm1__data.qcow2",
		Capacity:      20 << 30,
		Format:        "qcow2",
		BackingFile:   "/var/mnt/u-vms/images/base.qcow2",
		BackingFormat: "qcow2",
	}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec storageapi.StoragePoolVolumeSpecSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, res.TypedSpec().Pool, spec.GetPool())
	assert.Equal(t, res.TypedSpec().Name, spec.GetName())
	assert.Equal(t, res.TypedSpec().Capacity, spec.GetCapacity())
	assert.Equal(t, res.TypedSpec().Format, spec.GetFormat())
	assert.Equal(t, res.TypedSpec().BackingFile, spec.GetBackingFile())
	assert.Equal(t, res.TypedSpec().BackingFormat, spec.GetBackingFormat())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*storage.StoragePoolVolumeSpec).TypedSpec())
	assert.Equal(t, storage.NamespaceName, res.ResourceDefinition().DefaultNamespace)
}

func TestStoragePoolVolumeStatusRoundTrip(t *testing.T) {
	t.Parallel()

	res := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", "vm1__data.qcow2"))
	*res.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:            "pool1",
		Name:            "vm1__data.qcow2",
		Path:            "/var/mnt/u-vms/pool1/vm1__data.qcow2",
		Format:          "qcow2",
		Capacity:        20 << 30,
		PendingCapacity: 40 << 30,
		Phase:           storage.StoragePoolVolumePhaseReady,
		Error:           "waiting for the guest to stop before growing",
	}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec storageapi.StoragePoolVolumeStatusSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, res.TypedSpec().Pool, spec.GetPool())
	assert.Equal(t, res.TypedSpec().Name, spec.GetName())
	assert.Equal(t, res.TypedSpec().Path, spec.GetPath())
	assert.Equal(t, res.TypedSpec().Format, spec.GetFormat())
	assert.Equal(t, res.TypedSpec().Capacity, spec.GetCapacity())
	assert.Equal(t, res.TypedSpec().PendingCapacity, spec.GetPendingCapacity())
	assert.EqualValues(t, res.TypedSpec().Phase, spec.GetPhase())
	assert.Equal(t, res.TypedSpec().Error, spec.GetError())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*storage.StoragePoolVolumeStatus).TypedSpec())
}
