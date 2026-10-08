// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"reflect"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	hypervisorapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hypervisor"
	_ "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/storage"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

func TestDiskSourcePhaseContracts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		res        resource.Resource
		ready      int
		phaseField protoreflect.FieldNumber
		phases     []string
	}{
		{
			name:       "pool",
			res:        storage.NewStoragePoolStatus(storage.NamespaceName, "pool"),
			ready:      int(storage.StoragePoolPhaseReady),
			phaseField: 3,
			phases:     []string{"unknown", "notReady", "ready", "observationUnavailable"},
		},
		{
			name:       "volume",
			res:        storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "volume"),
			ready:      int(storage.StoragePoolVolumePhaseReady),
			phaseField: 7,
			phases:     []string{"unknown", "notReady", "ready", "observationUnavailable"},
		},
		{
			name:       "disk",
			res:        hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, "disk"),
			ready:      int(hypervisor.VirtualMachineDiskPhaseReady),
			phaseField: 6,
			phases:     []string{"unknown", "notReady", "ready", "observationUnavailable"},
		},
		{
			name:       "library",
			res:        hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "library"),
			ready:      int(hypervisor.ContentLibraryPhaseReady),
			phaseField: 3,
			phases:     []string{"unknown", "notReady", "ready"},
		},
		{
			name:       "seed",
			res:        hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, "seed"),
			ready:      int(hypervisor.CloudInitPhaseReady),
			phaseField: 10,
			phases:     []string{"unknown", "notReady", "ready"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := reflect.ValueOf(tt.res).MethodByName("TypedSpec").Call(nil)[0].Elem()
			assert.Zero(t, spec.FieldByName("Phase").Int())
			assert.False(t, spec.FieldByName("Ready").IsValid())
			assert.False(t, spec.FieldByName("ObservationUnavailable").IsValid())

			pkg := "hypervisor"
			if tt.name == "pool" || tt.name == "volume" {
				pkg = "storage"
			}

			message, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName("talos.resource.definitions." + pkg + "." + spec.Type().Name()))
			require.NoError(t, err)

			field := message.Descriptor().Fields().ByName("phase")
			require.NotNil(t, field)
			assert.Equal(t, protoreflect.EnumKind, field.Kind())
			assert.Equal(t, tt.phaseField, field.Number())

			assert.Zero(t, message.Descriptor().ReservedNames().Len())
			assert.Zero(t, message.Descriptor().ReservedRanges().Len())

			for i := range message.Descriptor().Fields().Len() {
				assert.Equal(t, protoreflect.FieldNumber(i+1), message.Descriptor().Fields().Get(i).Number())
			}

			encoded, err := protobuf.FromResource(tt.res)
			require.NoError(t, err)
			wire, err := encoded.Marshal()
			require.NoError(t, err)

			decoded, err := protobuf.Unmarshal(wire)
			require.NoError(t, err)
			roundTrip, err := protobuf.UnmarshalResource(decoded)
			require.NoError(t, err)
			data, err := yaml.Marshal(roundTrip.Spec())
			require.NoError(t, err)
			assert.Contains(t, string(data), "phase: unknown")
			assert.NotContains(t, string(data), "ready:")
			assert.NotContains(t, string(data), "observationUnavailable:")

			for i, phase := range tt.phases {
				wire.Spec.ProtoSpec = protowire.AppendTag(nil, tt.phaseField, protowire.VarintType)
				wire.Spec.ProtoSpec = protowire.AppendVarint(wire.Spec.ProtoSpec, uint64(i))
				decoded, err = protobuf.Unmarshal(wire)
				require.NoError(t, err)
				roundTrip, err = protobuf.UnmarshalResource(decoded)
				require.NoError(t, err)
				data, err = yaml.Marshal(roundTrip.Spec())
				require.NoError(t, err)
				assert.Contains(t, string(data), "phase: "+phase)

				reencoded, err := protobuf.FromResource(roundTrip)
				require.NoError(t, err)
				again, err := reencoded.Marshal()
				require.NoError(t, err)

				if i == 0 {
					assert.Empty(t, again.Spec.ProtoSpec, "proto3 omits the zero enum")
				} else {
					assert.Equal(t, wire.Spec.ProtoSpec, again.Spec.ProtoSpec)
				}

				yamlSpec := reflect.New(spec.Type())
				require.NoError(t, yaml.Unmarshal(data, yamlSpec.Interface()))
				assert.Equal(t, int64(i), yamlSpec.Elem().FieldByName("Phase").Int())
			}

			assert.Equal(t, 2, tt.ready)
		})
	}
}

func TestContentLibraryStatusRoundTrip(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	*res.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID:    "IMAGES",
		Path:        "/var/mnt/images",
		Phase:       hypervisor.ContentLibraryPhaseReady,
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
	assert.EqualValues(t, res.TypedSpec().Phase, spec.GetPhase())
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
