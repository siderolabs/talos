// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"
	"time"

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
		&containers.ContainerInstanceSpec{},
		&containers.ContainerInstanceStatus{},
		&containers.ContainerMountStatus{},
		&containers.ContainerLifecycle{},
		&containers.ContainerStatus{},
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

// TestInstanceProtobufRoundTrip guards the protobuf tags on ContainerInstanceSpecSpec.
func TestInstanceProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	spec := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID("nginx", 3))
	*spec.TypedSpec() = containers.ContainerInstanceSpecSpec{
		ContainerID: "nginx",
		Generation:  3,
		Image:       "docker.io/library/nginx@sha256:abc123",
		Entrypoint:  []string{"/entrypoint.sh"},
		Args:        []string{"nginx", "-g", "daemon off;"},
		WorkingDir:  "/srv",
		RunAs: containers.ContainerRunAsSpec{
			UID: new(int32(65534)),
			GID: new(int32(65534)),
		},
		Environment: []string{"NGINX_PORT=8080"},
		Mounts: []containers.ResolvedMountSpec{
			{
				Kind:        containers.MountKindHostPath,
				Source:      "/dev",
				Destination: "/dev",
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
	}

	assertRoundTrip(t, spec)
}

// TestInstanceStatusProtobufRoundTrip guards the protobuf tags on ContainerInstanceStatusSpec,
// including that the phase enum and timestamps survive the trip as more than their zero values.
func TestInstanceStatusProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	status := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID("nginx", 3))
	*status.TypedSpec() = containers.ContainerInstanceStatusSpec{
		ContainerID: "nginx",
		Generation:  3,
		Phase:       containers.ContainerInstancePhaseTerminated,
		PID:         1234,
		ExitCode:    137,
		Error:       "signal: killed",
		StartedAt:   time.Unix(1700000000, 0).UTC(),
		FinishedAt:  time.Unix(1700000123, 0).UTC(),
	}

	assertRoundTrip(t, status)
}

// TestMountStatusProtobufRoundTrip guards the protobuf tags on ContainerMountStatusSpec.
func TestMountStatusProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	status := containers.NewContainerMountStatus(containers.NamespaceName, "nginx")
	*status.TypedSpec() = containers.ContainerMountStatusSpec{
		Ready: true,
		Mounts: []containers.ResolvedMountSpec{
			{
				Kind:        containers.MountKindUserVolume,
				Source:      "/var/mnt/web-content",
				Destination: "/usr/share/nginx/html",
				Options:     []string{"ro"},
				VolumeID:    "u-web-content",
			},
			{
				Kind:        containers.MountKindTmpfs,
				Destination: "/tmp",
				Size:        64 << 20,
			},
		},
		Error: "volume \"u-other\" is not mounted",
	}

	assertRoundTrip(t, status)
}

// TestStatusProtobufRoundTrip guards the protobuf tags on ContainerStatusSpec, including that the
// state and health enums survive the trip as more than their zero values.
func TestStatusProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	status := containers.NewContainerStatus(containers.NamespaceName, "nginx")
	*status.TypedSpec() = containers.ContainerStatusSpec{
		State:        containers.ContainerStateBackoff,
		Health:       containers.ContainerHealthDegraded,
		Image:        "docker.io/library/nginx@sha256:abc123",
		PID:          1234,
		ExitCode:     137,
		RestartCount: 3,
		Error:        "signal: killed",
		WaitingFor:   []string{"container: other"},
	}

	assertRoundTrip(t, status)
}

// TestLifecycleProtobufRoundTrip guards the (empty) protobuf tags on ContainerLifecycleSpec.
func TestLifecycleProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	lifecycle := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID)

	assertRoundTrip(t, lifecycle)
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
