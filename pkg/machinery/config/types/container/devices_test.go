// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/container"
)

func TestContainerConfigDevices(t *testing.T) {
	t.Parallel()

	cfg := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: osd
image: quay.io/ceph/ceph:v20.2.4
mounts:
  - tmpfs:
      destination: /var/lib/ceph/osd
devices:
  - rawVolume:
      name: osd-1
      destination: /dev/osd-1
`)

	_, err := cfg.Validate(validationMode{})
	require.NoError(t, err)

	devices := cfg.Devices()
	require.Len(t, devices, 1)

	rawVolume, ok := devices[0].RawVolume().Get()
	require.True(t, ok)
	assert.Equal(t, "osd-1", rawVolume.Name())
	assert.Equal(t, "/dev/osd-1", rawVolume.Destination())
}

func TestValidateDeviceDestination(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		path        string
		expectedErr string
	}{
		{
			name: "device",
			path: "/dev/data",
		},
		{
			name: "nested device",
			path: "/dev/disk/data",
		},
		{
			// Only the entries the runtime creates are reserved, not names starting the same way.
			name: "name with a reserved prefix",
			path: "/dev/nullable",
		},
		{
			name:        "empty",
			path:        "",
			expectedErr: "rawVolume.destination is required",
		},
		{
			name:        "relative",
			path:        "dev/data",
			expectedErr: `rawVolume.destination "dev/data" must be an absolute path`,
		},
		{
			name:        "not clean",
			path:        "/dev/../dev/data",
			expectedErr: `rawVolume.destination "/dev/../dev/data" must be a clean path, i.e. "/dev/data"`,
		},
		{
			name:        "trailing slash",
			path:        "/dev/data/",
			expectedErr: `rawVolume.destination "/dev/data/" must be a clean path, i.e. "/dev/data"`,
		},
		{
			name:        "outside of /dev",
			path:        "/data",
			expectedErr: `rawVolume.destination "/data" must be under "/dev/"`,
		},
		{
			name:        "/dev itself",
			path:        "/dev",
			expectedErr: `rawVolume.destination "/dev" must be under "/dev/"`,
		},
		{
			name:        "default device",
			path:        "/dev/null",
			expectedErr: `rawVolume.destination "/dev/null" is reserved by the container runtime`,
		},
		{
			name:        "symlink to /proc",
			path:        "/dev/stdout",
			expectedErr: `rawVolume.destination "/dev/stdout" is reserved by the container runtime`,
		},
		{
			name:        "under a runtime mount",
			path:        "/dev/shm/data",
			expectedErr: `rawVolume.destination "/dev/shm/data" is reserved by the container runtime`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := container.ValidateDeviceDestination("rawVolume.destination", test.path)

			if test.expectedErr == "" {
				require.NoError(t, err)

				return
			}

			require.EqualError(t, err, test.expectedErr)
		})
	}
}

func TestContainerConfigValidateDevices(t *testing.T) {
	t.Parallel()

	rawVolume := func(name, destination string) container.ContainerDevice {
		return container.ContainerDevice{
			RawVolumeDevice: &container.RawVolumeDevice{VolumeName: name, DeviceDestination: destination},
		}
	}

	for _, test := range []struct {
		name         string
		mounts       []container.ContainerMount
		devices      []container.ContainerDevice
		expectedErrs []string
	}{
		{
			name: "no devices",
		},
		{
			name: "distinct devices",
			devices: []container.ContainerDevice{
				rawVolume("data-1", "/dev/data-1"),
				rawVolume("data-2", "/dev/data-2"),
			},
		},
		{
			name: "mounts beside devices",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/tmp"}},
				{HostPathMount: &container.HostPathMount{MountSource: "/dev/net/tun", MountDestination: "/dev/net/tun"}},
			},
			devices: []container.ContainerDevice{
				rawVolume("data", "/dev/data"),
			},
		},
		{
			name: "device with no source",
			devices: []container.ContainerDevice{
				{},
			},
			expectedErrs: []string{"devices[0]: exactly one of rawVolume must be set"},
		},
		{
			name: "raw volume with no name",
			devices: []container.ContainerDevice{
				rawVolume("", "/dev/data"),
			},
			expectedErrs: []string{"devices[0]: rawVolume.name is required"},
		},
		{
			name: "raw volume handed out twice",
			devices: []container.ContainerDevice{
				rawVolume("data", "/dev/data-1"),
				rawVolume("data", "/dev/data-2"),
			},
			expectedErrs: []string{`devices[1]: raw volume "data" is already handed to the container by devices[0]`},
		},
		{
			name: "duplicate destinations",
			devices: []container.ContainerDevice{
				rawVolume("data-1", "/dev/data"),
				rawVolume("data-2", "/dev/data"),
			},
			expectedErrs: []string{`devices[1]: destination "/dev/data" overlaps devices[0] destination "/dev/data"`},
		},
		{
			name: "nested destinations",
			devices: []container.ContainerDevice{
				rawVolume("data-1", "/dev/data"),
				rawVolume("data-2", "/dev/data/2"),
			},
			expectedErrs: []string{`devices[1]: destination "/dev/data/2" overlaps devices[0] destination "/dev/data"`},
		},
		{
			// The runtime keeps a path which already exists, so the device node would never be created.
			name: "mount at the device destination",
			mounts: []container.ContainerMount{
				{HostPathMount: &container.HostPathMount{MountSource: "/dev/sdb", MountDestination: "/dev/data"}},
			},
			devices: []container.ContainerDevice{
				rawVolume("data", "/dev/data"),
			},
			expectedErrs: []string{`devices[0]: destination "/dev/data" overlaps mounts[0] destination "/dev/data"`},
		},
		{
			name: "host /dev mounted over the devices",
			mounts: []container.ContainerMount{
				{HostPathMount: &container.HostPathMount{MountSource: "/dev", MountDestination: "/dev"}},
			},
			devices: []container.ContainerDevice{
				rawVolume("data", "/dev/data"),
			},
			expectedErrs: []string{`devices[0]: destination "/dev/data" overlaps mounts[0] destination "/dev"`},
		},
		{
			name: "mount destination compared clean",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/dev/disk/"}},
			},
			devices: []container.ContainerDevice{
				rawVolume("data", "/dev/disk/data"),
			},
			expectedErrs: []string{`devices[0]: destination "/dev/disk/data" overlaps mounts[0] destination "/dev/disk"`},
		},
		{
			name: "mount under a device",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/dev/data/scratch"}},
			},
			devices: []container.ContainerDevice{
				rawVolume("data", "/dev/data"),
			},
			expectedErrs: []string{`devices[0]: destination "/dev/data" overlaps mounts[0] destination "/dev/data/scratch"`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := &container.ContainerConfigV1Alpha1{MountsConfig: test.mounts, DevicesConfig: test.devices}

			err := cfg.ValidateDevices()

			if len(test.expectedErrs) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, expected := range test.expectedErrs {
				assert.Contains(t, err.Error(), expected)
			}
		})
	}
}
