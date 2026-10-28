// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// TestCheckImageUser covers which image USER values need a runAs override.
//
// Each half of USER given as a name must be overridden; a bare name also needs the gid, since that
// would be the user's primary group from /etc/passwd.
func TestCheckImageUser(t *testing.T) {
	t.Parallel()

	uid := containers.ContainerRunAsSpec{UID: new(int32(1000))}
	gid := containers.ContainerRunAsSpec{GID: new(int32(1000))}
	both := containers.ContainerRunAsSpec{UID: new(int32(1000)), GID: new(int32(1000))}

	for _, test := range []struct {
		name        string
		user        string
		runAs       containers.ContainerRunAsSpec
		wantMissing string // empty means accepted
	}{
		{name: "unset", user: ""},
		{name: "numeric uid", user: "1000"},
		{name: "numeric uid and gid", user: "1000:1001"},
		{name: "zero", user: "0:0"},
		{name: "max uint32", user: "4294967295"},
		{name: "uid out of range", user: "4294967296", wantMissing: "runAs.uid and runAs.gid"},
		{name: "negative uid", user: "-1", wantMissing: "runAs.uid and runAs.gid"},

		{name: "name", user: "app", wantMissing: "runAs.uid and runAs.gid"},
		{name: "name with uid", user: "app", runAs: uid, wantMissing: "runAs.gid"},
		{name: "name with gid", user: "app", runAs: gid, wantMissing: "runAs.uid"},
		{name: "name with both", user: "app", runAs: both},

		{name: "name and group", user: "app:app", wantMissing: "runAs.uid and runAs.gid"},
		{name: "name and group with both", user: "app:app", runAs: both},

		{name: "name and numeric gid", user: "app:1000", wantMissing: "runAs.uid"},
		{name: "name and numeric gid with uid", user: "app:1000", runAs: uid},

		{name: "numeric uid and group", user: "1000:app", wantMissing: "runAs.gid"},
		{name: "numeric uid and group with gid", user: "1000:app", runAs: gid},

		{name: "empty gid half", user: "1000:", wantMissing: "runAs.gid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := containersctrl.CheckImageUser(test.user, test.runAs)

			if test.wantMissing == "" {
				assert.NoError(t, err)

				return
			}

			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), "set "+test.wantMissing+" in the ContainerConfig document")
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
			name: "volume",
			mounts: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindVolume,
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
				{Kind: containers.MountKindVolume, Source: "/var/mnt/data", Destination: "/mnt/data"},
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

// TestMountsRequestSharedPropagation covers detection of a mount asking for rshared
// propagation: missing this flips the rootfs into slave propagation, so a container-created
// mount never becomes visible on the host.
func TestMountsRequestSharedPropagation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		mounts   []containers.ResolvedMountSpec
		expected bool
	}{
		{
			name: "none",
		},
		{
			name: "mount with no options",
			mounts: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log"},
			},
		},
		{
			name: "unrelated option only",
			mounts: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log", Options: []string{"ro"}},
			},
		},
		{
			name: "rshared option",
			mounts: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log", Options: []string{"rshared"}},
			},
			expected: true,
		},
		{
			name: "later mount requests rshared",
			mounts: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log", Options: []string{"ro"}},
				{Kind: containers.MountKindHostPath, Source: "/var/mnt", Destination: "/host-mnt", Options: []string{"rshared"}},
			},
			expected: true,
		},
		{
			name: "rshared is not the first option",
			mounts: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log", Options: []string{"rbind", "rshared"}},
			},
			expected: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, containersctrl.MountsRequestSharedPropagation(test.mounts))
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

func TestWithResolvedDevices(t *testing.T) {
	t.Parallel()

	var hostDevice unix.Stat_t

	require.NoError(t, unix.Stat("/dev/null", &hostDevice))

	spec := &specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				Devices: []specs.LinuxDeviceCgroup{{Allow: false, Access: "rwm"}},
			},
		},
	}

	err := containersctrl.WithResolvedDevices([]containers.ResolvedDeviceSpec{
		{
			Kind:        containers.DeviceKindRawVolume,
			Source:      "/dev/null",
			Destination: "/dev/data",
			VolumeID:    "r-data",
			VolumeName:  "data",
		},
	})(t.Context(), nil, nil, spec)
	require.NoError(t, err)

	major, minor := int64(unix.Major(hostDevice.Rdev)), int64(unix.Minor(hostDevice.Rdev)) //nolint:gosec

	require.Len(t, spec.Linux.Devices, 1)
	assert.Equal(t, "/dev/data", spec.Linux.Devices[0].Path)
	assert.Equal(t, major, spec.Linux.Devices[0].Major)
	assert.Equal(t, minor, spec.Linux.Devices[0].Minor)

	// Appended after the default deny-all rule, so only this device is allowed.
	require.Len(t, spec.Linux.Resources.Devices, 2)
	assert.Equal(t, specs.LinuxDeviceCgroup{
		Allow:  true,
		Type:   spec.Linux.Devices[0].Type,
		Major:  &major,
		Minor:  &minor,
		Access: "rw",
	}, spec.Linux.Resources.Devices[1])
}

func TestWithResolvedDevicesMissingSource(t *testing.T) {
	t.Parallel()

	spec := &specs.Spec{Linux: &specs.Linux{Resources: &specs.LinuxResources{}}}

	err := containersctrl.WithResolvedDevices([]containers.ResolvedDeviceSpec{
		{Kind: containers.DeviceKindRawVolume, Source: "/dev/does-not-exist", Destination: "/dev/data"},
	})(t.Context(), nil, nil, spec)
	require.ErrorContains(t, err, `failed to add device "/dev/data"`)
}
