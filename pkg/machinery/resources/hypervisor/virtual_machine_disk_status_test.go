// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hypervisorapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineDiskStatusRoundTrip(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, "guest/system@0123456789ab")
	*res.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: "guest",
		Name:           "system",
		SourcePath:     "/var/mnt/images/alpine.iso",
		Format:         "raw",
		ReadOnly:       true,
		Ready:          true,
		Error:          "",
		Image: hypervisor.VirtualMachineDiskFromImageSpec{
			Library: "images",
			File:    "alpine.iso",
			Digest:  "sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Mode:    "cdrom",
		},
	}

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.VirtualMachineDiskStatusSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, res.TypedSpec().VirtualMachine, spec.GetVirtualMachine())
	assert.Equal(t, res.TypedSpec().Name, spec.GetName())
	assert.Equal(t, res.TypedSpec().SourcePath, spec.GetSourcePath())
	assert.Equal(t, res.TypedSpec().Format, spec.GetFormat())
	assert.Equal(t, res.TypedSpec().ReadOnly, spec.GetReadOnly())
	assert.Equal(t, res.TypedSpec().Ready, spec.GetReady())
	assert.Equal(t, res.TypedSpec().Image.Library, spec.GetImage().GetLibrary())
	assert.Equal(t, res.TypedSpec().Image.File, spec.GetImage().GetFile())
	assert.Equal(t, res.TypedSpec().Image.Digest, spec.GetImage().GetDigest())
	assert.Equal(t, res.TypedSpec().Image.Mode, spec.GetImage().GetMode())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*hypervisor.VirtualMachineDiskStatus).TypedSpec())

	clone := res.DeepCopy().(*hypervisor.VirtualMachineDiskStatus)
	clone.TypedSpec().Image.File = "changed"
	assert.NotEqual(t, clone.TypedSpec().Image.File, res.TypedSpec().Image.File)
	assert.Equal(t, hypervisor.NamespaceName, res.ResourceDefinition().DefaultNamespace)
}

// VirtualMachineDiskStatusID spells its fields out, and the ID is what tells a status provisioned
// from one image apart from a status provisioned from another: a field it does not cover is a
// difference domains would not notice.
func TestVirtualMachineDiskStatusIDCoversEveryImageField(t *testing.T) {
	t.Parallel()

	require.Equal(t, 4, reflect.TypeFor[hypervisor.VirtualMachineDiskFromImageSpec]().NumField(),
		"VirtualMachineDiskFromImageSpec gained a field: add it to VirtualMachineDiskStatusID")

	base := hypervisor.VirtualMachineDiskFromImageSpec{
		Library: "images",
		File:    "alpine.iso",
		Digest:  "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Mode:    "cdrom",
	}

	id := func(image hypervisor.VirtualMachineDiskFromImageSpec) string {
		return hypervisor.VirtualMachineDiskStatusID("guest", hypervisor.VirtualMachineDiskSpec{
			Name:      "system",
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{FromImage: &image},
		})
	}

	for name, change := range map[string]func(*hypervisor.VirtualMachineDiskFromImageSpec){
		"library": func(image *hypervisor.VirtualMachineDiskFromImageSpec) { image.Library = "other" },
		"file":    func(image *hypervisor.VirtualMachineDiskFromImageSpec) { image.File = "other.iso" },
		"digest": func(image *hypervisor.VirtualMachineDiskFromImageSpec) {
			image.Digest = "sha256:" + strings.Repeat("f", 64)
		},
		"mode": func(image *hypervisor.VirtualMachineDiskFromImageSpec) { image.Mode = "disk" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			changed := base
			change(&changed)

			assert.NotEqual(t, id(base), id(changed))
		})
	}

	// A disk provisioned from no image hashes the zero value, so it still has one stable ID to
	// report its own unsupportedness under.
	assert.Equal(t,
		hypervisor.VirtualMachineDiskStatusID("guest", hypervisor.VirtualMachineDiskSpec{Name: "system"}),
		id(hypervisor.VirtualMachineDiskFromImageSpec{}),
	)
}
