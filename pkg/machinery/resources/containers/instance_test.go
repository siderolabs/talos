// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestProcessEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    *containers.ContainerSpecSpec
		i    *containers.ContainerInstanceSpecSpec
		want bool
	}{
		{
			name: "equal",
			s: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			i: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			want: true,
		},
		{
			name: "entrypoint differs",
			s: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			i: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/bash"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			want: false,
		},
		{
			name: "args differs",
			s: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			i: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg2"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			want: false,
		},
		{
			name: "workingDir differs",
			s: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			i: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/other",
				Environment: []string{"KEY=VAL"},
			},
			want: false,
		},
		{
			name: "environment differs",
			s: &containers.ContainerSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=VAL"},
			},
			i: &containers.ContainerInstanceSpecSpec{
				Entrypoint:  []string{"/bin/sh"},
				Args:        []string{"arg1"},
				WorkingDir:  "/work",
				Environment: []string{"KEY=OTHER"},
			},
			want: false,
		},
		{
			name: "nil and empty slices are equal",
			s: &containers.ContainerSpecSpec{
				Entrypoint:  nil,
				Args:        nil,
				WorkingDir:  "/work",
				Environment: nil,
			},
			i: &containers.ContainerInstanceSpecSpec{
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

			got := containers.ProcessEqual(tt.s, tt.i)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSecurityEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    containers.ContainerSecuritySpec
		b    containers.ContainerSecuritySpec
		want bool
	}{
		{
			name: "equal",
			a: containers.ContainerSecuritySpec{
				Privileged:       true,
				CapabilitiesAdd:  []string{"NET_ADMIN"},
				CapabilitiesDrop: []string{"ALL"},
			},
			b: containers.ContainerSecuritySpec{
				Privileged:       true,
				CapabilitiesAdd:  []string{"NET_ADMIN"},
				CapabilitiesDrop: []string{"ALL"},
			},
			want: true,
		},
		{
			name: "privileged differs",
			a: containers.ContainerSecuritySpec{
				Privileged: true,
			},
			b: containers.ContainerSecuritySpec{
				Privileged: false,
			},
			want: false,
		},
		{
			name: "capabilitiesAdd differs",
			a: containers.ContainerSecuritySpec{
				CapabilitiesAdd: []string{"NET_ADMIN"},
			},
			b: containers.ContainerSecuritySpec{
				CapabilitiesAdd: []string{"SYS_ADMIN"},
			},
			want: false,
		},
		{
			name: "capabilitiesDrop differs",
			a: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"ALL"},
			},
			b: containers.ContainerSecuritySpec{
				CapabilitiesDrop: []string{"NET_RAW"},
			},
			want: false,
		},
		{
			name: "nil and empty slices are equal",
			a: containers.ContainerSecuritySpec{
				CapabilitiesAdd:  nil,
				CapabilitiesDrop: nil,
			},
			b: containers.ContainerSecuritySpec{
				CapabilitiesAdd:  []string{},
				CapabilitiesDrop: []string{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.SecurityEqual(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunAsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    containers.ContainerRunAsSpec
		b    containers.ContainerRunAsSpec
		want bool
	}{
		{
			name: "both nil UID and GID",
			a:    containers.ContainerRunAsSpec{UID: nil, GID: nil},
			b:    containers.ContainerRunAsSpec{UID: nil, GID: nil},
			want: true,
		},
		{
			name: "both UID set equal",
			a:    containers.ContainerRunAsSpec{UID: new(int32(1000))},
			b:    containers.ContainerRunAsSpec{UID: new(int32(1000))},
			want: true,
		},
		{
			name: "both UID set different",
			a:    containers.ContainerRunAsSpec{UID: new(int32(1000))},
			b:    containers.ContainerRunAsSpec{UID: new(int32(2000))},
			want: false,
		},
		{
			name: "UID nil vs set",
			a:    containers.ContainerRunAsSpec{UID: nil},
			b:    containers.ContainerRunAsSpec{UID: new(int32(1000))},
			want: false,
		},
		{
			name: "both GID set equal",
			a:    containers.ContainerRunAsSpec{GID: new(int32(1000))},
			b:    containers.ContainerRunAsSpec{GID: new(int32(1000))},
			want: true,
		},
		{
			name: "both GID set different",
			a:    containers.ContainerRunAsSpec{GID: new(int32(1000))},
			b:    containers.ContainerRunAsSpec{GID: new(int32(2000))},
			want: false,
		},
		{
			name: "GID nil vs set",
			a:    containers.ContainerRunAsSpec{GID: nil},
			b:    containers.ContainerRunAsSpec{GID: new(int32(1000))},
			want: false,
		},
		{
			name: "UID and GID both set equal",
			a:    containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(1001))},
			b:    containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(1001))},
			want: true,
		},
		{
			name: "UID equal but GID differs",
			a:    containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(1001))},
			b:    containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(2001))},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.RunAsEqual(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInt32PtrEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    *int32
		b    *int32
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "a nil b set",
			a:    nil,
			b:    new(int32(42)),
			want: false,
		},
		{
			name: "a set b nil",
			a:    new(int32(42)),
			b:    nil,
			want: false,
		},
		{
			name: "both set equal",
			a:    new(int32(42)),
			b:    new(int32(42)),
			want: true,
		},
		{
			name: "both set different",
			a:    new(int32(42)),
			b:    new(int32(99)),
			want: false,
		},
		{
			name: "zero values equal",
			a:    new(int32(0)),
			b:    new(int32(0)),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.Int32PtrEqual(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolvedMountsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []containers.ResolvedMountSpec
		b    []containers.ResolvedMountSpec
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "nil and empty are equal",
			a:    nil,
			b:    []containers.ResolvedMountSpec{},
			want: true,
		},
		{
			name: "both empty",
			a:    []containers.ResolvedMountSpec{},
			b:    []containers.ResolvedMountSpec{},
			want: true,
		},
		{
			name: "equal single mount",
			a: []containers.ResolvedMountSpec{
				{
					Kind:        "hostPath",
					Source:      "/dev",
					Destination: "/host/dev",
					Size:        0,
					Options:     []string{"ro"},
				},
			},
			b: []containers.ResolvedMountSpec{
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
			a: []containers.ResolvedMountSpec{
				{Kind: "hostPath"},
			},
			b: []containers.ResolvedMountSpec{
				{Kind: "tmpfs"},
			},
			want: false,
		},
		{
			name: "different Source",
			a: []containers.ResolvedMountSpec{
				{Source: "/dev"},
			},
			b: []containers.ResolvedMountSpec{
				{Source: "/sys"},
			},
			want: false,
		},
		{
			name: "different Destination",
			a: []containers.ResolvedMountSpec{
				{Destination: "/host/dev"},
			},
			b: []containers.ResolvedMountSpec{
				{Destination: "/dev"},
			},
			want: false,
		},
		{
			name: "different Size",
			a: []containers.ResolvedMountSpec{
				{Size: 64 << 20},
			},
			b: []containers.ResolvedMountSpec{
				{Size: 128 << 20},
			},
			want: false,
		},
		{
			name: "different Options",
			a: []containers.ResolvedMountSpec{
				{Options: []string{"ro"}},
			},
			b: []containers.ResolvedMountSpec{
				{Options: []string{"rw"}},
			},
			want: false,
		},
		{
			name: "nil and empty Options are equal",
			a: []containers.ResolvedMountSpec{
				{Options: nil},
			},
			b: []containers.ResolvedMountSpec{
				{Options: []string{}},
			},
			want: true,
		},
		{
			name: "equal multiple mounts",
			a: []containers.ResolvedMountSpec{
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
			b: []containers.ResolvedMountSpec{
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
			a: []containers.ResolvedMountSpec{
				{Kind: "hostPath", Destination: "/dev"},
				{Kind: "tmpfs", Destination: "/tmp"},
			},
			b: []containers.ResolvedMountSpec{
				{Kind: "tmpfs", Destination: "/tmp"},
				{Kind: "hostPath", Destination: "/dev"},
			},
			want: false,
		},
		{
			name: "different length",
			a: []containers.ResolvedMountSpec{
				{Kind: "hostPath"},
			},
			b: []containers.ResolvedMountSpec{
				{Kind: "hostPath"},
				{Kind: "tmpfs"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.ResolvedMountsEqual(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveInstanceMounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mounts    []containers.ContainerMountSpec
		wantReady bool
		wantCount int
	}{
		{
			name:      "no mounts",
			mounts:    []containers.ContainerMountSpec{},
			wantReady: true,
			wantCount: 0,
		},
		{
			name: "tmpfs mount",
			mounts: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
					Size:        1024,
				},
			},
			wantReady: true,
			wantCount: 1,
		},
		{
			name: "hostpath mount",
			mounts: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var",
					Destination: "/data",
				},
			},
			wantReady: true,
			wantCount: 1,
		},
		{
			name: "user volume mount",
			mounts: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindUserVolume,
					Destination: "/mnt",
				},
			},
			wantReady: false,
			wantCount: 0,
		},
		{
			name: "mixed mounts with user volume",
			mounts: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
				},
				{
					Kind:        containers.MountKindUserVolume,
					Destination: "/mnt",
				},
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var",
					Destination: "/data",
				},
			},
			wantReady: false,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ready := containers.ResolveInstanceMounts(tt.mounts)
			assert.Equal(t, tt.wantReady, ready)
			assert.Equal(t, tt.wantCount, len(got))
		})
	}
}
