// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// TestMountsResolvedMatchDeclared covers the freshness check callers run against
// GetResolvedMountStatus' result: every field a resolution must not change has to match, or a stale
// status must be rejected.
func TestMountsResolvedMatchDeclared(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		resolved []containers.ResolvedMountSpec
		declared []containers.ContainerMountSpec
		resolves bool
	}{
		{
			name: "matches",
			resolved: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var/log",
					Destination: "/host-log",
					Options:     []string{"ro"},
				},
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
					Size:        64 << 20,
				},
			},
			declared: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var/log",
					Destination: "/host-log",
					Options:     []string{"ro"},
				},
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
					Size:        64 << 20,
				},
			},
			resolves: true,
		},
		{
			// The bug this test guards: a declared options edit must invalidate a resolution taken
			// before the edit, not just a kind/destination change.
			name: "options changed",
			resolved: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var/log",
					Destination: "/host-log",
				},
			},
			declared: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var/log",
					Destination: "/host-log",
					Options:     []string{"ro"},
				},
			},
			resolves: false,
		},
		{
			name: "tmpfs size changed",
			resolved: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
					Size:        64 << 20,
				},
			},
			declared: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
					Size:        128 << 20,
				},
			},
			resolves: false,
		},
		{
			name: "hostPath source changed",
			resolved: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var/log",
					Destination: "/host-log",
				},
			},
			declared: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/var/log/audit",
					Destination: "/host-log",
				},
			},
			resolves: false,
		},
		{
			// The resolved source is what MountController fills in from the mounted volume; the
			// declared side only ever carries a VolumeName, so the two are never compared for this kind.
			name: "volume source is not compared",
			resolved: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindVolume,
					Source:      "/var/mnt/web-content",
					Destination: "/usr/share/nginx/html",
					VolumeName:  "web-content",
				},
			},
			declared: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindVolume,
					VolumeName:  "web-content",
					Destination: "/usr/share/nginx/html",
				},
			},
			resolves: true,
		},
		{
			name: "volume volumeName changed",
			resolved: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindVolume,
					Source:      "/var/mnt/web-content",
					Destination: "/usr/share/nginx/html",
					VolumeName:  "web-content",
				},
			},
			declared: []containers.ContainerMountSpec{
				{
					Kind:        containers.MountKindVolume,
					VolumeName:  "other-volume",
					Destination: "/usr/share/nginx/html",
				},
			},
			resolves: false,
		},
		{
			name: "mount count mismatch",
			resolved: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindTmpfs, Destination: "/tmp"},
			},
			declared: nil,
			resolves: false,
		},
		{
			// Order is significant: when two mounts land on the same destination, the later one wins,
			// so a reordering can change which mount is actually visible.
			name: "reordered mounts do not resolve",
			resolved: []containers.ResolvedMountSpec{
				{Kind: containers.MountKindTmpfs, Destination: "/tmp", Size: 64 << 20},
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log"},
			},
			declared: []containers.ContainerMountSpec{
				{Kind: containers.MountKindHostPath, Source: "/var/log", Destination: "/host-log"},
				{Kind: containers.MountKindTmpfs, Destination: "/tmp", Size: 64 << 20},
			},
			resolves: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.resolves, containers.MountsResolvedMatchDeclared(test.resolved, test.declared))
		})
	}
}

func TestGetResolvedMountStatus(t *testing.T) {
	t.Parallel()

	const containerID = "nginx"

	mountSpecs := []containers.ResolvedMountSpec{
		{
			Kind:        containers.MountKindHostPath,
			Source:      "/var/log",
			Destination: "/host-log",
		},
	}

	deviceSpecs := []containers.ResolvedDeviceSpec{
		{
			Kind:        containers.DeviceKindRawVolume,
			Source:      "/dev/sdb1",
			Destination: "/dev/data",
			VolumeID:    "r-data",
			VolumeName:  "data",
		},
	}

	for _, test := range []struct {
		name   string
		create bool
		ready  bool
	}{
		{
			name:   "ready",
			create: true,
			ready:  true,
		},
		{
			name:   "not ready",
			create: true,
			ready:  false,
		},
		{
			name:   "not found",
			create: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resources := state.WrapCore(namespaced.NewState(inmem.Build))

			if test.create {
				status := containers.NewContainerMountStatus(containers.NamespaceName, containerID)
				*status.TypedSpec() = containers.ContainerMountStatusSpec{
					Ready:   test.ready,
					Mounts:  mountSpecs,
					Devices: deviceSpecs,
				}

				require.NoError(t, resources.Create(t.Context(), status))
			}

			resolved, err := containers.GetResolvedMountStatus(t.Context(), resources, containerID)
			require.NoError(t, err)

			if test.create && test.ready {
				assert.Equal(t, mountSpecs, resolved.Mounts)
				assert.Equal(t, deviceSpecs, resolved.Devices)
			} else {
				assert.Nil(t, resolved.Mounts)
				assert.Nil(t, resolved.Devices)
			}
		})
	}
}

func TestDevicesResolvedMatchDeclared(t *testing.T) {
	t.Parallel()

	declared := []containers.ContainerDeviceSpec{
		{Kind: containers.DeviceKindRawVolume, VolumeName: "data", Destination: "/dev/data"},
	}

	resolved := containers.ResolvedDeviceSpec{
		Kind:        containers.DeviceKindRawVolume,
		Source:      "/dev/dm-0",
		Destination: "/dev/data",
		VolumeID:    "r-data",
		VolumeName:  "data",
	}

	for _, test := range []struct {
		name     string
		resolved []containers.ResolvedDeviceSpec
		declared []containers.ContainerDeviceSpec
		resolves bool
	}{
		{
			name:     "none",
			resolves: true,
		},
		{
			name:     "same device",
			resolved: []containers.ResolvedDeviceSpec{resolved},
			declared: declared,
			resolves: true,
		},
		{
			name:     "not resolved yet",
			declared: declared,
		},
		{
			name:     "removed from the spec",
			resolved: []containers.ResolvedDeviceSpec{resolved},
		},
		{
			name: "other destination",
			resolved: []containers.ResolvedDeviceSpec{
				{Kind: containers.DeviceKindRawVolume, Source: "/dev/dm-0", Destination: "/dev/other", VolumeID: "r-data", VolumeName: "data"},
			},
			declared: declared,
		},
		{
			name: "other raw volume",
			resolved: []containers.ResolvedDeviceSpec{
				{Kind: containers.DeviceKindRawVolume, Source: "/dev/dm-1", Destination: "/dev/data", VolumeID: "r-other", VolumeName: "other"},
			},
			declared: declared,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.resolves, containers.DevicesResolvedMatchDeclared(test.resolved, test.declared))
		})
	}
}

func TestResolvedWaitingFor(t *testing.T) {
	t.Parallel()

	spec := containers.ContainerSpecSpec{
		Mounts: []containers.ContainerMountSpec{
			{Kind: containers.MountKindTmpfs, Destination: "/tmp"},
		},
		Devices: []containers.ContainerDeviceSpec{
			{Kind: containers.DeviceKindRawVolume, VolumeName: "data", Destination: "/dev/data"},
		},
	}

	mounts := []containers.ResolvedMountSpec{{Kind: containers.MountKindTmpfs, Destination: "/tmp"}}
	devices := []containers.ResolvedDeviceSpec{
		{Kind: containers.DeviceKindRawVolume, Source: "/dev/sdb1", Destination: "/dev/data", VolumeID: "r-data", VolumeName: "data"},
	}

	assert.Equal(t, []string{"mounts", "devices"}, spec.ResolvedWaitingFor(containers.ContainerMountStatusSpec{}))
	assert.Equal(t, []string{"devices"}, spec.ResolvedWaitingFor(containers.ContainerMountStatusSpec{Ready: true, Mounts: mounts}))
	assert.Empty(t, spec.ResolvedWaitingFor(containers.ContainerMountStatusSpec{Ready: true, Mounts: mounts, Devices: devices}))
	assert.True(t, spec.ResolvedMatchDeclared(containers.ContainerMountStatusSpec{Ready: true, Mounts: mounts, Devices: devices}))
}

func TestResolvedDevicesEqual(t *testing.T) {
	t.Parallel()

	device := containers.ResolvedDeviceSpec{
		Kind:        containers.DeviceKindRawVolume,
		Source:      "/dev/dm-0",
		Destination: "/dev/data",
		VolumeID:    "r-data",
		VolumeName:  "data",
	}

	reopened := device
	reopened.Source = "/dev/dm-1"

	assert.True(t, containers.ResolvedDevicesEqual(nil, []containers.ResolvedDeviceSpec{}))
	assert.True(t, containers.ResolvedDevicesEqual([]containers.ResolvedDeviceSpec{device}, []containers.ResolvedDeviceSpec{device}))
	// Opened again as another device: the instance has to be replaced.
	assert.False(t, containers.ResolvedDevicesEqual([]containers.ResolvedDeviceSpec{device}, []containers.ResolvedDeviceSpec{reopened}))
	assert.False(t, containers.ResolvedDevicesEqual([]containers.ResolvedDeviceSpec{device}, nil))
}
