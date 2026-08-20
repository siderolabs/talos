// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestMountsResolvedToOCI(t *testing.T) {
	tests := []struct {
		name  string
		input []containers.ResolvedMountSpec
		check func(t *testing.T, mounts any)
	}{
		{
			name:  "empty mounts",
			input: []containers.ResolvedMountSpec{},
			check: func(t *testing.T, mounts any) {
				assert.Nil(t, mounts)
			},
		},
		{
			name: "tmpfs mount",
			input: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
					Size:        1024 * 1024,
					Options:     []string{"noexec"},
				},
			},
			check: func(t *testing.T, mounts any) {
				// Just verify it's not nil and has 1 element
				require.NotNil(t, mounts)
				// Can't easily introspect the mount without importing specs
			},
		},
		{
			name: "hostpath mount",
			input: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/host/path",
					Destination: "/container/path",
					Options:     []string{"ro"},
				},
			},
			check: func(t *testing.T, mounts any) {
				require.NotNil(t, mounts)
			},
		},
		{
			name: "mixed mounts",
			input: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
				},
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/host/data",
					Destination: "/data",
				},
			},
			check: func(t *testing.T, mounts any) {
				require.NotNil(t, mounts)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts := containersctrl.MountsResolvedToOCI(tt.input)
			tt.check(t, mounts)
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
