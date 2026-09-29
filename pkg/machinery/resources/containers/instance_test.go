// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestProcessEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containerSpec *containers.ContainerSpecSpec
		instanceSpec  *containers.ContainerInstanceSpecSpec
		want          bool
	}{
		{
			name: "equal",
			containerSpec: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			instanceSpec: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			want: true,
		},
		{
			name: "entrypoint differs",
			containerSpec: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			instanceSpec: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/bash"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			want: false,
		},
		{
			name: "args differs",
			containerSpec: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			instanceSpec: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg2"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			want: false,
		},
		{
			name: "workingDir differs",
			containerSpec: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			instanceSpec: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/other",
				Environment: []string{"KEY=VAL"},
			},
			want: false,
		},
		{
			name: "environment differs",
			containerSpec: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			instanceSpec: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=OTHER"},
			},
			want: false,
		},
		{
			name: "nil and empty slices are equal",
			containerSpec: &containers.ContainerSpecSpec{
				Entrypoint:  nil,
				Args:        nil,
				WorkingDir:  "/work",
				Environment: nil,
			},
			instanceSpec: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{},
				Args:        []string{},
				WorkingDir:  "/work",
				Environment: []string{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.containerSpec.InstanceProcessEqual(*tt.instanceSpec)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSecurityEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		securitySpecA containers.ContainerSecuritySpec
		securitySpecB containers.ContainerSecuritySpec
		want          bool
	}{
		{
			name: "equal",
			securitySpecA: containers.ContainerSecuritySpec{
				Privileged:       true,
				CapabilitiesAdd:  []string{"NET_ADMIN"},
				CapabilitiesDrop: []string{"ALL"},
			},
			securitySpecB: containers.ContainerSecuritySpec{
				Privileged:       true,
				CapabilitiesAdd:  []string{"NET_ADMIN"},
				CapabilitiesDrop: []string{"ALL"},
			},
			want: true,
		},
		{
			name: "privileged differs",
			securitySpecA: containers.ContainerSecuritySpec{
				Privileged: true,
			},
			securitySpecB: containers.ContainerSecuritySpec{
				Privileged: false,
			},
			want: false,
		},
		{
			name: "capabilitiesAdd differs",
			securitySpecA: containers.ContainerSecuritySpec{
				CapabilitiesAdd: []string{"NET_ADMIN"},
			},
			securitySpecB: containers.ContainerSecuritySpec{
				CapabilitiesAdd: []string{"SYS_ADMIN"},
			},
			want: false,
		},
		{
			name: "capabilitiesDrop differs",
			securitySpecA: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"ALL"},
			},
			securitySpecB: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"NET_RAW"},
			},
			want: false,
		},
		{
			name: "nil and empty slices are equal",
			securitySpecA: containers.ContainerSecuritySpec{
				CapabilitiesAdd:  nil,
				CapabilitiesDrop: nil,
			},
			securitySpecB: containers.ContainerSecuritySpec{
				CapabilitiesAdd:  []string{},
				CapabilitiesDrop: []string{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.securitySpecA.Equal(tt.securitySpecB)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunAsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		runAsSpecA containers.ContainerRunAsSpec
		runAsSpecB containers.ContainerRunAsSpec
		want       bool
	}{
		{
			name:       "both nil UID and GID",
			runAsSpecA: containers.ContainerRunAsSpec{UID: nil, GID: nil},
			runAsSpecB: containers.ContainerRunAsSpec{UID: nil, GID: nil},
			want:       true,
		},
		{
			name:       "both UID set equal",
			runAsSpecA: containers.ContainerRunAsSpec{UID: new(int32(1000))},
			runAsSpecB: containers.ContainerRunAsSpec{UID: new(int32(1000))},
			want:       true,
		},
		{
			name:       "both UID set different",
			runAsSpecA: containers.ContainerRunAsSpec{UID: new(int32(1000))},
			runAsSpecB: containers.ContainerRunAsSpec{UID: new(int32(2000))},
			want:       false,
		},
		{
			name:       "UID nil vs set",
			runAsSpecA: containers.ContainerRunAsSpec{UID: nil},
			runAsSpecB: containers.ContainerRunAsSpec{UID: new(int32(1000))},
			want:       false,
		},
		{
			name:       "both GID set equal",
			runAsSpecA: containers.ContainerRunAsSpec{GID: new(int32(1000))},
			runAsSpecB: containers.ContainerRunAsSpec{GID: new(int32(1000))},
			want:       true,
		},
		{
			name:       "both GID set different",
			runAsSpecA: containers.ContainerRunAsSpec{GID: new(int32(1000))},
			runAsSpecB: containers.ContainerRunAsSpec{GID: new(int32(2000))},
			want:       false,
		},
		{
			name:       "GID nil vs set",
			runAsSpecA: containers.ContainerRunAsSpec{GID: nil},
			runAsSpecB: containers.ContainerRunAsSpec{GID: new(int32(1000))},
			want:       false,
		},
		{
			name:       "UID and GID both set equal",
			runAsSpecA: containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(1001))},
			runAsSpecB: containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(1001))},
			want:       true,
		},
		{
			name:       "UID equal but GID differs",
			runAsSpecA: containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(1001))},
			runAsSpecB: containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(2001))},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.runAsSpecA.Equal(tt.runAsSpecB)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInt32PtrEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		int32A *int32
		int32B *int32
		want   bool
	}{
		{
			name:   "both nil",
			int32A: nil,
			int32B: nil,
			want:   true,
		},
		{
			name:   "a nil b set",
			int32A: nil,
			int32B: new(int32(42)),
			want:   false,
		},
		{
			name:   "a set b nil",
			int32A: new(int32(42)),
			int32B: nil,
			want:   false,
		},
		{
			name:   "both set equal",
			int32A: new(int32(42)),
			int32B: new(int32(42)),
			want:   true,
		},
		{
			name:   "both set different",
			int32A: new(int32(42)),
			int32B: new(int32(99)),
			want:   false,
		},
		{
			name:   "zero values equal",
			int32A: new(int32(0)),
			int32B: new(int32(0)),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.Int32PtrEqual(tt.int32A, tt.int32B)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolvedMountsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		resolvedMountSpecA []containers.ResolvedMountSpec
		resolvedMountSpecB []containers.ResolvedMountSpec
		want               bool
	}{
		{
			name:               "both nil",
			resolvedMountSpecA: nil,
			resolvedMountSpecB: nil,
			want:               true,
		},
		{
			name:               "nil and empty are equal",
			resolvedMountSpecA: nil,
			resolvedMountSpecB: []containers.ResolvedMountSpec{},
			want:               true,
		},
		{
			name:               "both empty",
			resolvedMountSpecA: []containers.ResolvedMountSpec{},
			resolvedMountSpecB: []containers.ResolvedMountSpec{},
			want:               true,
		},
		{
			name: "equal single mount",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{
					Kind:        "hostPath",
					Source:      "/dev",
					Destination: "/host/dev",
					Size:        0,
					Options:     []string{"ro"},
				},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{
					Kind:        "hostPath",
					Source:      "/dev",
					Destination: "/host/dev",
					Size:        0,
					Options:     []string{"ro"},
				},
			},
			want: true,
		},
		{
			name: "different Kind",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Kind: "hostPath"},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Kind: "tmpfs"},
			},
			want: false,
		},
		{
			name: "different Source",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Source: "/dev"},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Source: "/sys"},
			},
			want: false,
		},
		{
			name: "different Destination",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Destination: "/host/dev"},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Destination: "/dev"},
			},
			want: false,
		},
		{
			name: "different Size",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Size: 64 << 20},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Size: 128 << 20},
			},
			want: false,
		},
		{
			name: "different Options",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Options: []string{"ro"}},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Options: []string{"rw"}},
			},
			want: false,
		},
		{
			name: "different VolumeID",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{VolumeID: "u-web-content"},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{VolumeID: "u-other-volume"},
			},
			want: false,
		},
		{
			name: "nil and empty Options are equal",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Options: nil},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Options: []string{}},
			},
			want: true,
		},
		{
			name: "equal multiple mounts",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{
					Kind:        "hostPath",
					Source:      "/dev",
					Destination: "/host/dev",
					Options:     []string{"ro"},
				},
				{
					Kind:        "tmpfs",
					Destination: "/tmp",
					Size:        64 << 20,
					Options:     []string{"nosuid"},
				},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{
					Kind:        "hostPath",
					Source:      "/dev",
					Destination: "/host/dev",
					Options:     []string{"ro"},
				},
				{
					Kind:        "tmpfs",
					Destination: "/tmp",
					Size:        64 << 20,
					Options:     []string{"nosuid"},
				},
			},
			want: true,
		},
		{
			name: "same mounts different order are not equal (order-sensitive)",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Kind: "hostPath", Destination: "/dev"},
				{Kind: "tmpfs", Destination: "/tmp"},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Kind: "tmpfs", Destination: "/tmp"},
				{Kind: "hostPath", Destination: "/dev"},
			},
			want: false,
		},
		{
			name: "different length",
			resolvedMountSpecA: []containers.ResolvedMountSpec{
				{Kind: "hostPath"},
			},
			resolvedMountSpecB: []containers.ResolvedMountSpec{
				{Kind: "hostPath"},
				{Kind: "tmpfs"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.ResolvedMountsEqual(tt.resolvedMountSpecA, tt.resolvedMountSpecB)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeriveReportedError(t *testing.T) {
	t.Parallel()

	newInstanceStatus := func(errMsg string) *containers.ContainerInstanceStatus {
		status := containers.NewContainerInstanceStatus(containers.NamespaceName, "nginx-0")
		status.TypedSpec().Error = errMsg

		return status
	}

	newImageStatus := func(errMsg string) *containers.ContainerImageStatus {
		status := containers.NewContainerImageStatus(containers.NamespaceName, "nginx")
		status.TypedSpec().Error = errMsg

		return status
	}

	tests := []struct {
		name                    string
		containerInstanceStatus *containers.ContainerInstanceStatus
		containerImageStatus    *containers.ContainerImageStatus
		want                    string
	}{
		{
			name: "neither present",
		},
		{
			name:                    "instance error only",
			containerInstanceStatus: newInstanceStatus("instance failed"),
			want:                    "instance failed",
		},
		{
			name:                 "image error only",
			containerImageStatus: newImageStatus("pull failed"),
			want:                 "pull failed",
		},
		{
			name:                    "instance error outranks image error",
			containerInstanceStatus: newInstanceStatus("instance failed"),
			containerImageStatus:    newImageStatus("pull failed"),
			want:                    "instance failed",
		},
		{
			name:                    "instance present without an error falls back to the image error",
			containerInstanceStatus: newInstanceStatus(""),
			containerImageStatus:    newImageStatus("pull failed"),
			want:                    "pull failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.DeriveReportedError(tt.containerInstanceStatus, tt.containerImageStatus)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLatestInstanceStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	const containerID = "nginx"

	newStatus := func(generation uint64) *containers.ContainerInstanceStatus {
		status := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID(containerID, generation))
		status.TypedSpec().ContainerID = containerID
		status.TypedSpec().Generation = generation
		status.Metadata().Labels().Set(containers.ContainerSpecIdLabel, containerID)

		return status
	}

	// Created out of generation order: a bug that returned the last-listed resource rather than
	// tracking the maximum Generation would not be caught by a monotonic creation order.
	require.NoError(t, st.Create(ctx, newStatus(1)))
	require.NoError(t, st.Create(ctx, newStatus(3)))
	require.NoError(t, st.Create(ctx, newStatus(0)))
	require.NoError(t, st.Create(ctx, newStatus(2)))

	got, err := containers.LatestInstanceStatus(ctx, st, containerID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint64(3), got.TypedSpec().Generation)
}

func TestLatestInstanceStatusNone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	got, err := containers.LatestInstanceStatus(ctx, st, "nginx")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCurrentInstanceSpec(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	const containerID = "nginx"

	t.Run("nil status returns nil", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))

		got, err := containers.CurrentInstanceSpec(ctx, st, containerID, nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("status without a matching spec returns nil", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))

		status := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID(containerID, 0))
		status.TypedSpec().Generation = 0

		got, err := containers.CurrentInstanceSpec(ctx, st, containerID, status)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns the spec matching the status's generation", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))

		spec := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(containerID, 2))
		spec.TypedSpec().ContainerID = containerID
		require.NoError(t, st.Create(ctx, spec))

		status := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID(containerID, 2))
		status.TypedSpec().Generation = 2

		got, err := containers.CurrentInstanceSpec(ctx, st, containerID, status)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, containers.InstanceID(containerID, 2), got.Metadata().ID())
	})
}
