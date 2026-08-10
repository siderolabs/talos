// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestRegisterResource(t *testing.T) {
	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, res := range []meta.ResourceWithRD{
		&containers.ContainerSpec{},
		&containers.ContainerImageStatus{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, res))
	}
}

// TestProtobufRoundTrip guards the protobuf tags on ContainerSpecSpec. RegisterDynamic marshals via
// those tags rather than generated code, so a missing or duplicated tag only surfaces here or on
// the wire.
func TestProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	spec := containers.NewContainerSpec(containers.NamespaceName, "nginx")
	*spec.TypedSpec() = containers.ContainerSpecSpec{
		Image:      containers.ContainerImageSpec{Ref: "docker.io/library/nginx:latest"},
		Entrypoint: []string{"/entrypoint.sh"},
		Args:       []string{"nginx", "-g", "daemon off;"},
		WorkingDir: "/srv",
		RunAs: containers.ContainerRunAsSpec{
			UID: new(int32(65534)),
			GID: new(int32(65534)),
		},
		Environment: []string{"NGINX_PORT=8080"},
		Mounts: []containers.ContainerMountSpec{
			{
				Kind:        containers.MountKindUserVolume,
				VolumeID:    "u-web-content",
				Destination: "/usr/share/nginx/html",
				Options:     []string{"ro"},
			},
			{
				Kind:        containers.MountKindTmpfs,
				Destination: "/tmp",
				Size:        64 << 20,
			},
		},
		Security: containers.ContainerSecuritySpec{
			Privileged:       true,
			CapabilitiesAdd:  []string{"NET_ADMIN"},
			CapabilitiesDrop: []string{"ALL"},
		},
		Network:   containers.ContainerNetworkSpec{HostNetwork: true},
		Resources: containers.ContainerResourcesSpec{MemoryLimit: 1 << 29, CPULimit: 1500},
		DependsOn: containers.ContainerDependsOnSpec{
			Paths:      []string{"/var/mnt/web-content"},
			Networks:   []string{"addresses"},
			Time:       true,
			Containers: []string{"other"},
		},
	}

	assertRoundTrip(t, spec)
}

// TestImageStatusProtobufRoundTrip guards the protobuf tags on ContainerImageStatusSpec, including
// that the phase enum survives the trip as more than its zero value.
func TestImageStatusProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	status := containers.NewContainerImageStatus(containers.NamespaceName, "nginx")
	*status.TypedSpec() = containers.ContainerImageStatusSpec{
		Phase:  containers.ContainerImagePhaseFailed,
		Image:  "docker.io/library/nginx:latest",
		Digest: "sha256:abc123",
		Error:  "signature verification denied",
	}

	assertRoundTrip(t, status)
}

func assertRoundTrip[T resource.Resource](t *testing.T, res T) {
	t.Helper()

	protoRes, err := protobuf.FromResource(res)
	require.NoError(t, err)

	marshaled, err := protoRes.Marshal()
	require.NoError(t, err)

	unmarshaled, err := protobuf.Unmarshal(marshaled)
	require.NoError(t, err)

	decoded, err := protobuf.UnmarshalResource(unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, res.Metadata().ID(), decoded.Metadata().ID())
	assert.Equal(t, res.Metadata().Type(), decoded.Metadata().Type())
	assert.Equal(t, res.Spec(), decoded.Spec())
}
