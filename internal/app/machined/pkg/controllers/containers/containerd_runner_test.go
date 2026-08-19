// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestParseNumericUser(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantUID   uint32
		wantGID   uint32
		wantValid bool
	}{
		{
			name:      "uid only",
			input:     "1000",
			wantUID:   1000,
			wantGID:   0,
			wantValid: true,
		},
		{
			name:      "uid and gid",
			input:     "1000:1001",
			wantUID:   1000,
			wantGID:   1001,
			wantValid: true,
		},
		{
			name:      "zero uid",
			input:     "0",
			wantUID:   0,
			wantGID:   0,
			wantValid: true,
		},
		{
			name:      "zero uid and gid",
			input:     "0:0",
			wantUID:   0,
			wantGID:   0,
			wantValid: true,
		},
		{
			name:      "invalid uid",
			input:     "abc",
			wantValid: false,
		},
		{
			name:      "invalid gid",
			input:     "1000:abc",
			wantValid: false,
		},
		{
			name:      "empty string",
			input:     "",
			wantValid: false,
		},
		{
			name:      "max uint32",
			input:     "4294967295",
			wantUID:   4294967295,
			wantGID:   0,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, gid, valid := containersctrl.ParseNumericUser(tt.input)
			assert.Equal(t, tt.wantValid, valid)

			if valid {
				assert.Equal(t, tt.wantUID, uid)
				assert.Equal(t, tt.wantGID, gid)
			}
		})
	}
}

// TestMountsResolvedToOCI covers the translation of every resolved mount kind.
//
// Every kind has to be handled: a kind that falls through is dropped silently, and the container then
// runs without a mount it declared, which is only visible once something inside it looks for the
// destination.
func TestMountsResolvedToOCI(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		mounts   []containers.ResolvedMountSpec
		expected []specs.Mount
	}{
		{
			name: "none",
		},
		{
			name: "tmpfs with a size",
			mounts: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/scratch",
					Size:        1024,
				},
			},
			expected: []specs.Mount{
				{
					Type:        "tmpfs",
					Source:      "tmpfs",
					Destination: "/scratch",
					Options:     []string{"nosuid", "nodev", "size=1024"},
				},
			},
		},
		{
			name: "host path, read-only",
			mounts: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var/log",
					Destination: "/host-log",
					Options:     []string{"ro"},
				},
			},
			expected: []specs.Mount{
				{
					Type:        "bind",
					Source:      "/var/log",
					Destination: "/host-log",
					Options:     []string{"rbind", "ro"},
				},
			},
		},
		{
			// The source is the path the volume is mounted at, filled in by MountController.
			name: "user volume",
			mounts: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindUserVolume,
					Source:      "/var/mnt/data",
					Destination: "/mnt/data",
				},
			},
			expected: []specs.Mount{
				{
					Type:        "bind",
					Source:      "/var/mnt/data",
					Destination: "/mnt/data",
					Options:     []string{"rbind"},
				},
			},
		},
		{
			name: "all kinds keep their declared order",
			mounts: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindUserVolume, Source: "/var/mnt/data", Destination: "/mnt/data"},
				{Kind: containers.MountKindTmpfs, Destination: "/scratch"},
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log"},
			},
			expected: []specs.Mount{
				{Type: "bind", Source: "/var/mnt/data", Destination: "/mnt/data", Options: []string{"rbind"}},
				{Type: "tmpfs", Source: "tmpfs", Destination: "/scratch", Options: []string{"nosuid", "nodev"}},
				{Type: "bind", Source: "/var/log", Destination: "/host-log", Options: []string{"rbind"}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, containersctrl.MountsResolvedToOCI(test.mounts))
		})
	}
}

func TestWithProcessArgs(t *testing.T) {
	// Test that WithProcessArgs returns an oci.SpecOpts (function)
	// We can't easily test the full behavior without mocking the image,
	// but we can verify it returns a function
	t.Run("returns function", func(t *testing.T) {
		spec := containers.ContainerInstanceSpecSpec{
			Entrypoint: []string{"/bin/sh"},
			Args:       []string{"-c", "echo hello"},
		}

		// Just verify this doesn't panic and returns something
		// Full testing would require image mock
		fn := containersctrl.WithProcessArgs(spec, nil)
		assert.NotNil(t, fn)
	})
}

func TestWithRunAs(t *testing.T) {
	// Test that WithRunAs returns an oci.SpecOpts (function)
	t.Run("returns function", func(t *testing.T) {
		var (
			uid int32 = 1000
			gid int32 = 1001
		)

		spec := containers.ContainerRunAsSpec{
			UID: &uid,
			GID: &gid,
		}

		fn := containersctrl.WithRunAs(spec)
		assert.NotNil(t, fn)
	})

	t.Run("nil uid and gid", func(t *testing.T) {
		spec := containers.ContainerRunAsSpec{
			UID: nil,
			GID: nil,
		}

		fn := containersctrl.WithRunAs(spec)
		assert.NotNil(t, fn)
	})
}
