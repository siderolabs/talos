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

func TestValidateMountOptions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		options     []string
		expectedErr string
	}{
		{
			name:        "nil options",
			options:     nil,
			expectedErr: "",
		},
		{
			name:        "empty options",
			options:     []string{},
			expectedErr: "",
		},
		{
			name:        "single valid option ro",
			options:     []string{"ro"},
			expectedErr: "",
		},
		{
			name:        "single valid option rw",
			options:     []string{"rw"},
			expectedErr: "",
		},
		{
			name:        "single valid option noexec",
			options:     []string{"noexec"},
			expectedErr: "",
		},
		{
			name:        "single valid option nosuid",
			options:     []string{"nosuid"},
			expectedErr: "",
		},
		{
			name:        "single valid option nodev",
			options:     []string{"nodev"},
			expectedErr: "",
		},
		{
			name:        "single valid option noatime",
			options:     []string{"noatime"},
			expectedErr: "",
		},
		{
			name:        "single valid option rbind",
			options:     []string{"rbind"},
			expectedErr: "",
		},
		{
			name:        "single valid option rshared",
			options:     []string{"rshared"},
			expectedErr: "",
		},
		{
			name:        "all valid options",
			options:     []string{"ro", "noexec", "nosuid", "nodev", "noatime", "rbind", "rshared"},
			expectedErr: "",
		},
		{
			name:        "duplicate valid option",
			options:     []string{"ro", "ro"},
			expectedErr: "",
		},
		{
			name:        "case sensitive - uppercase RO",
			options:     []string{"RO"},
			expectedErr: `unsupported mount option "RO"`,
		},
		{
			name:        "single unsupported option",
			options:     []string{"sync"},
			expectedErr: `unsupported mount option "sync"`,
		},
		{
			name:        "multiple unsupported options",
			options:     []string{"sync", "async"},
			expectedErr: "unsupported mount option", // will verify both options in error
		},
		{
			name:        "ro and rw together",
			options:     []string{"ro", "rw"},
			expectedErr: "mutually exclusive",
		},
		{
			name:        "rw and ro together",
			options:     []string{"rw", "ro"},
			expectedErr: "mutually exclusive",
		},
		{
			name:        "ro and rw with other valid option",
			options:     []string{"ro", "rw", "noexec"},
			expectedErr: "mutually exclusive",
		},
		{
			name:        "ro and rw with unsupported option",
			options:     []string{"ro", "rw", "sync"},
			expectedErr: "mutually exclusive", // will verify both errors present
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := container.ValidateMountOptions(test.options)

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				// For the "multiple unsupported options" case, verify both are in the error
				if test.name == "multiple unsupported options" {
					assert.Contains(t, err.Error(), `"sync"`)
					assert.Contains(t, err.Error(), `"async"`)
				}

				// For the "ro and rw with unsupported option" case, verify both errors
				if test.name == "ro and rw with unsupported option" {
					assert.Contains(t, err.Error(), "mutually exclusive")
					assert.Contains(t, err.Error(), `"sync"`)
				}

				// For all other cases, just verify the expected substring
				if test.name != "multiple unsupported options" && test.name != "ro and rw with unsupported option" {
					assert.Contains(t, err.Error(), test.expectedErr)
				}
			}
		})
	}
}

func TestContainerMountValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                string
		mount               container.ContainerMount
		expectedDestination string
		expectedErr         string
	}{
		{
			name:        "no source set",
			mount:       container.ContainerMount{},
			expectedErr: "exactly one of userVolume, tmpfs or hostPath must be set",
		},
		{
			name: "two sources set",
			mount: container.ContainerMount{
				TmpfsMount:    &container.TmpfsMount{MountDestination: "/tmp"},
				HostPathMount: &container.HostPathMount{MountSource: "/dev", MountDestination: "/dev"},
			},
			expectedErr: "exactly one of userVolume, tmpfs or hostPath must be set",
		},
		{
			name: "three sources set",
			mount: container.ContainerMount{
				UserVolumeMount: &container.UserVolumeMount{VolumeName: "data", MountDestination: "/data"},
				TmpfsMount:      &container.TmpfsMount{MountDestination: "/tmp"},
				HostPathMount:   &container.HostPathMount{MountSource: "/dev", MountDestination: "/dev"},
			},
			expectedErr: "exactly one of userVolume, tmpfs or hostPath must be set",
		},
		{
			name: "valid userVolume",
			mount: container.ContainerMount{
				UserVolumeMount: &container.UserVolumeMount{VolumeName: "data", MountDestination: "/var/lib/data"},
			},
			expectedDestination: "/var/lib/data",
		},
		{
			name: "valid tmpfs",
			mount: container.ContainerMount{
				TmpfsMount: &container.TmpfsMount{MountDestination: "/tmp"},
			},
			expectedDestination: "/tmp",
		},
		{
			name: "valid hostPath",
			mount: container.ContainerMount{
				HostPathMount: &container.HostPathMount{MountSource: "/dev", MountDestination: "/dev"},
			},
			expectedDestination: "/dev",
		},
		{
			name: "invalid userVolume still returns its destination",
			mount: container.ContainerMount{
				UserVolumeMount: &container.UserVolumeMount{MountDestination: "/var/lib/data"},
			},
			expectedDestination: "/var/lib/data",
			expectedErr:         "userVolume.name is required",
		},
		{
			name: "invalid tmpfs still returns its destination",
			mount: container.ContainerMount{
				TmpfsMount: &container.TmpfsMount{MountDestination: "tmp"},
			},
			expectedDestination: "tmp",
			expectedErr:         "tmpfs.destination",
		},
		{
			name: "invalid hostPath still returns its destination",
			mount: container.ContainerMount{
				HostPathMount: &container.HostPathMount{MountSource: "dev", MountDestination: "/dev"},
			},
			expectedDestination: "/dev",
			expectedErr:         "hostPath.source",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			destination, err := test.mount.Validate()

			assert.Equal(t, test.expectedDestination, destination)

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
			}
		})
	}
}

func TestUserVolumeMountValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		mount       container.UserVolumeMount
		expectedErr string
	}{
		{
			name: "valid",
			mount: container.UserVolumeMount{
				VolumeName:       "data",
				MountDestination: "/var/lib/data",
			},
		},
		{
			name: "missing name",
			mount: container.UserVolumeMount{
				MountDestination: "/var/lib/data",
			},
			expectedErr: "userVolume.name is required",
		},
		{
			name: "relative destination",
			mount: container.UserVolumeMount{
				VolumeName:       "data",
				MountDestination: "var/lib/data",
			},
			expectedErr: "userVolume.destination",
		},
		{
			name: "unsupported mount option",
			mount: container.UserVolumeMount{
				VolumeName:       "data",
				MountDestination: "/var/lib/data",
				MountOpts:        []string{"sync"},
			},
			expectedErr: `unsupported mount option "sync"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.mount.Validate()

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
			}
		})
	}
}

func TestTmpfsMountValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		mount       container.TmpfsMount
		expectedErr string
	}{
		{
			name: "valid",
			mount: container.TmpfsMount{
				MountDestination: "/tmp",
			},
		},
		{
			name: "relative destination",
			mount: container.TmpfsMount{
				MountDestination: "tmp",
			},
			expectedErr: "tmpfs.destination",
		},
		{
			name: "unsupported mount option",
			mount: container.TmpfsMount{
				MountDestination: "/tmp",
				MountOpts:        []string{"sync"},
			},
			expectedErr: `unsupported mount option "sync"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.mount.Validate()

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
			}
		})
	}
}

func TestHostPathMountValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		mount       container.HostPathMount
		expectedErr string
	}{
		{
			name: "valid",
			mount: container.HostPathMount{
				MountSource:      "/dev",
				MountDestination: "/dev",
			},
		},
		{
			name: "relative source",
			mount: container.HostPathMount{
				MountSource:      "dev",
				MountDestination: "/dev",
			},
			expectedErr: "hostPath.source",
		},
		{
			name: "relative destination",
			mount: container.HostPathMount{
				MountSource:      "/dev",
				MountDestination: "dev",
			},
			expectedErr: "hostPath.destination",
		},
		{
			name: "unsupported mount option",
			mount: container.HostPathMount{
				MountSource:      "/dev",
				MountDestination: "/dev",
				MountOpts:        []string{"sync"},
			},
			expectedErr: `unsupported mount option "sync"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.mount.Validate()

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
			}
		})
	}
}
