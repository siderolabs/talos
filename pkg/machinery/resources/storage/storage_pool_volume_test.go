// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"testing"
	"time"

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
		Copy: &storage.StoragePoolVolumeCopySpec{
			Source:        "/library/objects/sha256/abc",
			Digest:        "sha256:abc",
			SourceLibrary: "images",
			Image:         true,
		},
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
	require.NotNil(t, spec.GetCopy())
	assert.NotNil(t, spec.GetCopy().ProtoReflect().Descriptor().Fields().ByName("source_library"), "wire contract must retain the source dependency")

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
		Copy:            &storage.StoragePoolVolumeCopyStatusSpec{Phase: storage.StoragePoolVolumeCopyPhaseSucceeded, Intent: "intent"},
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
	assert.NotNil(t, spec.ProtoReflect().Descriptor().Fields().ByName("copy"), "wire contract must include copy observation")

	cloned := res.DeepCopy().(*storage.StoragePoolVolumeStatus)
	cloned.TypedSpec().Copy.Intent = "other"
	assert.Equal(t, "intent", res.TypedSpec().Copy.Intent, "deep copy must isolate copy observation")

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*storage.StoragePoolVolumeStatus).TypedSpec())
}

func TestCopyPhaseContract(t *testing.T) {
	t.Parallel()
	//nolint:misspell // Public copy phase spelling.
	phases := []string{"unknown", "waiting", "queued", "running", "cancelling", "succeeded", "failed"}
	for value, name := range phases {
		phase := storage.StoragePoolVolumeCopyPhase(value)
		assert.Equal(t, name, phase.String())
	}

	for _, phase := range []storage.StoragePoolVolumeCopyPhase{0, 1, 2, 3, 4, 5, 6, 127} {
		res := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "pool/copy")
		res.TypedSpec().Copy = &storage.StoragePoolVolumeCopyStatusSpec{Phase: phase, Intent: "intent"}
		encoded, err := protobuf.FromResource(res)
		require.NoError(t, err)
		wire, err := encoded.Marshal()
		require.NoError(t, err)

		var spec storageapi.StoragePoolVolumeStatusSpec
		require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
		require.NotNil(t, spec.GetCopy())
		assert.EqualValues(t, phase, spec.GetCopy().GetPhase())
		wire.Spec.ProtoSpec, err = proto.Marshal(&spec)
		require.NoError(t, err)
		decoded, err := protobuf.Unmarshal(wire)
		require.NoError(t, err)
		roundTrip, err := protobuf.UnmarshalResource(decoded)
		require.NoError(t, err)
		assert.Equal(t, res.TypedSpec(), roundTrip.(*storage.StoragePoolVolumeStatus).TypedSpec())

		if phase == 0 || phase == 127 {
			assert.NotEqual(t, storage.StoragePoolVolumeCopyPhaseSucceeded, phase)
			assert.NotEqual(t, storage.StoragePoolVolumeCopyPhaseFailed, phase)
		}
	}
}

func TestCopyIntentIsSemantic(t *testing.T) {
	t.Parallel()

	res := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "pool/copy")
	res.TypedSpec().Copy = &storage.StoragePoolVolumeCopySpec{Source: "source", Digest: "digest"}
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool")
	pool.TypedSpec().VolumeID = "backing"
	intent := storage.StoragePoolVolumeCopyIntent(res, pool)
	res.Metadata().Labels().Set("metadata", "changed")
	assert.NotEmpty(t, intent)

	changedPool := pool.DeepCopy().(*storage.StoragePoolStatus)
	changedPool.TypedSpec().VolumeID = "new backing"
	assert.NotEqual(t, intent, storage.StoragePoolVolumeCopyIntent(res, changedPool), "backing identity qualifies observations")
	incarnation := res.DeepCopy().(*storage.StoragePoolVolumeSpec)
	incarnation.Metadata().SetCreated(res.Metadata().Created().Add(time.Second))
	assert.NotEqual(t, intent, storage.StoragePoolVolumeCopyIntent(incarnation, pool), "resource incarnation qualifies observations")

	assert.Equal(t, intent, storage.StoragePoolVolumeCopyIntent(res, pool))
	clone := res.DeepCopy().(*storage.StoragePoolVolumeSpec)
	clone.TypedSpec().Copy.Source = "replacement"
	assert.NotEqual(t, intent, storage.StoragePoolVolumeCopyIntent(clone, pool))
	clone.TypedSpec().Copy = nil
	assert.Empty(t, storage.StoragePoolVolumeCopyIntent(clone, pool))
}

func TestWritableCopyCompletionPreservesOnlyCapacityEdits(t *testing.T) {
	t.Parallel()

	for name, change := range map[string]func(*storage.StoragePoolVolumeSpec, *storage.StoragePoolStatus){
		"capacity": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) { r.TypedSpec().Capacity *= 2 },
		"source": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) {
			r.TypedSpec().Copy.Source = "other"
		},
		"digest": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) {
			r.TypedSpec().Copy.Digest = "other"
		},
		"library": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) {
			r.TypedSpec().Copy.SourceLibrary = "other"
		},
		"image":  func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) { r.TypedSpec().Copy.Image = false },
		"format": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) { r.TypedSpec().Format = "raw" },
		"backing": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) {
			r.TypedSpec().BackingFile = "other"
		},
		"backing format": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) {
			r.TypedSpec().BackingFormat = "raw"
		},
		"name": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) { r.TypedSpec().Name = "other" },
		"pool": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) { r.TypedSpec().Pool = "other" },
		"request incarnation": func(r *storage.StoragePoolVolumeSpec, _ *storage.StoragePoolStatus) {
			r.Metadata().SetCreated(r.Metadata().Created().Add(time.Second))
		},
		"pool incarnation": func(_ *storage.StoragePoolVolumeSpec, p *storage.StoragePoolStatus) {
			p.Metadata().SetCreated(p.Metadata().Created().Add(time.Second))
		},
		"pool target": func(_ *storage.StoragePoolVolumeSpec, p *storage.StoragePoolStatus) {
			p.TypedSpec().TargetPath = "/other"
		},
		"pool backing": func(_ *storage.StoragePoolVolumeSpec, p *storage.StoragePoolStatus) { p.TypedSpec().VolumeID = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "pool/guest.img")
			*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
				Pool: "pool", Name: "guest.img", Capacity: 20 << 30, Format: "qcow2",
				Copy: &storage.StoragePoolVolumeCopySpec{Image: true, Source: "/library/object", Digest: "sha256:123", SourceLibrary: "images"},
			}
			pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool")
			pool.TypedSpec().TargetPath, pool.TypedSpec().VolumeID = "/pool", "backing"
			observed := &storage.StoragePoolVolumeCopyStatusSpec{
				Phase:  storage.StoragePoolVolumeCopyPhaseSucceeded,
				Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
			}
			workerIntent := storage.StoragePoolVolumeCopyIntent(request, pool)
			require.True(t, storage.StoragePoolVolumeCopyCompletionMatches(request, pool, observed))
			// Include a capacity edit with every unsafe change: initialization must
			// still check every other part of the original publication proof.
			change(request, pool)

			if name != "capacity" {
				request.TypedSpec().Capacity *= 2
			}

			assert.NotEqual(t, workerIntent, storage.StoragePoolVolumeCopyIntent(request, pool), "initial worker must be fenced")
			assert.Equal(t, name == "capacity", storage.StoragePoolVolumeCopyCompletionMatches(request, pool, observed))
			observed.Phase = storage.StoragePoolVolumeCopyPhaseRunning
			assert.False(t, storage.StoragePoolVolumeCopyCompletionMatches(request, pool, observed), "incomplete imports cannot qualify as completed initialization")
		})
	}
}
