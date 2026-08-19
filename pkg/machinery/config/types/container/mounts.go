// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"slices"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// validMountOptions are the mount options accepted on a container mount.
var validMountOptions = []string{
	"ro",
	"rw",
	"noexec",
	"nosuid",
	"nodev",
	"noatime",
	"rbind",
	"rshared",
}

// ContainerMount describes a single filesystem to mount into the container.
//
// Exactly one source must be set. Raw OCI mounts are deliberately not exposed; every source is
// typed so that Talos can reason about what a container is allowed to reach.
type ContainerMount struct {
	//   description: |
	//     Mount a user volume, referenced by the name of its `UserVolumeConfig` document.
	//
	//     The volume is mounted from `/var/mnt/<name>` on the host. Declaring this mount also
	//     makes the container wait for the volume to be mounted before it starts.
	UserVolumeMount *UserVolumeMount `yaml:"userVolume,omitempty"`
	//   description: |
	//     Mount a tmpfs for scratch space.
	TmpfsMount *TmpfsMount `yaml:"tmpfs,omitempty"`
	//   description: |
	//     Bind-mount a path from the host.
	//
	//     The source must already exist; Talos will not create it. This is the widest of the
	//     three sources and the only one that can reach arbitrary host state.
	HostPathMount *HostPathMount `yaml:"hostPath,omitempty"`
}

// UserVolumeMount mounts a user volume by name.
type UserVolumeMount struct {
	//   description: |
	//     Name of the `UserVolumeConfig` document to mount.
	VolumeName string `yaml:"name"`
	//   description: |
	//     Absolute path inside the container's mount namespace.
	MountDestination string `yaml:"destination"`
	//   description: |
	//     Mount options. User volume mounts are writable by default (`rw`).
	//   values:
	//     - ro
	//     - rw
	//     - noexec
	//     - nosuid
	//     - nodev
	//     - noatime
	//     - rbind
	//     - rshared
	MountOpts []string `yaml:"options,omitempty"`
}

// TmpfsMount mounts a tmpfs.
type TmpfsMount struct {
	//   description: |
	//     Absolute path inside the container's mount namespace.
	MountDestination string `yaml:"destination"`
	//   description: |
	//     Size of the tmpfs, e.g. `64MiB`. Empty means the kernel default.
	//   examples:
	//     - value: '"64MiB"'
	//   schema:
	//     type: string
	MountSize string `yaml:"size,omitempty"`
	//   description: |
	//     Mount options. Tmpfs mounts are writable by default (`rw`).
	MountOpts []string `yaml:"options,omitempty"`
}

// HostPathMount bind-mounts a host path.
type HostPathMount struct {
	//   description: |
	//     Absolute path on the host. Must already exist.
	MountSource string `yaml:"source"`
	//   description: |
	//     Absolute path inside the container's mount namespace.
	MountDestination string `yaml:"destination"`
	//   description: |
	//     Mount options. Host path mounts are writable by default (`rw`).
	MountOpts []string `yaml:"options,omitempty"`
}

// Validate checks the mount and returns its destination.
func (m *ContainerMount) Validate() (string, error) {
	matchCount := 0

	if m.UserVolumeMount != nil {
		matchCount++
	}

	if m.TmpfsMount != nil {
		matchCount++
	}

	if m.HostPathMount != nil {
		matchCount++
	}

	if matchCount != 1 {
		return "", errors.New("exactly one of userVolume, tmpfs or hostPath must be set")
	}

	var (
		destination string
		err         error
	)

	switch {
	case m.UserVolumeMount != nil:
		destination = m.UserVolumeMount.MountDestination
		err = m.UserVolumeMount.Validate()
	case m.TmpfsMount != nil:
		destination = m.TmpfsMount.MountDestination
		err = m.TmpfsMount.Validate()
	case m.HostPathMount != nil:
		destination = m.HostPathMount.MountDestination
		err = m.HostPathMount.Validate()
	}

	return destination, err
}

func (m *UserVolumeMount) Validate() error {
	var validationErrors error

	if m.VolumeName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("userVolume.name is required"))
	}

	return errors.Join(
		validationErrors,
		ValidateAbsPath("userVolume.destination", m.MountDestination),
		ValidateMountOptions(m.MountOpts),
	)
}

func (m *TmpfsMount) Validate() error {
	return errors.Join(
		ValidateAbsPath("tmpfs.destination", m.MountDestination),
		ValidateMountOptions(m.MountOpts),
	)
}

func (m *HostPathMount) Validate() error {
	return errors.Join(
		ValidateAbsPath("hostPath.source", m.MountSource),
		ValidateAbsPath("hostPath.destination", m.MountDestination),
		ValidateMountOptions(m.MountOpts),
	)
}

func ValidateMountOptions(options []string) error {
	var validationErrors error

	for _, option := range options {
		if !slices.Contains(validMountOptions, option) {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("unsupported mount option %q", option))
		}
	}

	if slices.Contains(options, "ro") && slices.Contains(options, "rw") {
		validationErrors = errors.Join(validationErrors, errors.New("mount options ro and rw are mutually exclusive"))
	}

	return validationErrors
}

// Check interfaces.
var (
	_ config.ContainerMountConfig           = &ContainerMount{}
	_ config.ContainerUserVolumeMountConfig = &UserVolumeMount{}
	_ config.ContainerTmpfsMountConfig      = &TmpfsMount{}
	_ config.ContainerHostPathMountConfig   = &HostPathMount{}
)

// UserVolume implements config.ContainerMountConfig interface.
func (m *ContainerMount) UserVolume() optional.Optional[config.ContainerUserVolumeMountConfig] {
	if m.UserVolumeMount == nil {
		return optional.None[config.ContainerUserVolumeMountConfig]()
	}

	return optional.Some[config.ContainerUserVolumeMountConfig](m.UserVolumeMount)
}

// Tmpfs implements config.ContainerMountConfig interface.
func (m *ContainerMount) Tmpfs() optional.Optional[config.ContainerTmpfsMountConfig] {
	if m.TmpfsMount == nil {
		return optional.None[config.ContainerTmpfsMountConfig]()
	}

	return optional.Some[config.ContainerTmpfsMountConfig](m.TmpfsMount)
}

// HostPath implements config.ContainerMountConfig interface.
func (m *ContainerMount) HostPath() optional.Optional[config.ContainerHostPathMountConfig] {
	if m.HostPathMount == nil {
		return optional.None[config.ContainerHostPathMountConfig]()
	}

	return optional.Some[config.ContainerHostPathMountConfig](m.HostPathMount)
}

// Name implements config.ContainerUserVolumeMountConfig interface.
func (m *UserVolumeMount) Name() string { return m.VolumeName }

// Destination implements config.ContainerUserVolumeMountConfig interface.
func (m *UserVolumeMount) Destination() string { return m.MountDestination }

// MountOptions implements config.ContainerUserVolumeMountConfig interface.
func (m *UserVolumeMount) MountOptions() []string { return normalizeWritableOptions(m.MountOpts) }

// Destination implements config.ContainerTmpfsMountConfig interface.
func (m *TmpfsMount) Destination() string { return m.MountDestination }

// Size implements config.ContainerTmpfsMountConfig interface.
func (m *TmpfsMount) Size() string { return m.MountSize }

// MountOptions implements config.ContainerTmpfsMountConfig interface.
func (m *TmpfsMount) MountOptions() []string { return normalizeWritableOptions(m.MountOpts) }

// Source implements config.ContainerHostPathMountConfig interface.
func (m *HostPathMount) Source() string { return m.MountSource }

// Destination implements config.ContainerHostPathMountConfig interface.
func (m *HostPathMount) Destination() string { return m.MountDestination }

// MountOptions implements config.ContainerHostPathMountConfig interface.
func (m *HostPathMount) MountOptions() []string { return normalizeWritableOptions(m.MountOpts) }

// normalizeWritableOptions applies the writable default shared by every container mount kind.
//
// An explicit `ro` is honored, and a redundant explicit `rw` is stripped since it's the default.
func normalizeWritableOptions(options []string) []string {
	return slices.DeleteFunc(slices.Clone(options), func(o string) bool { return o == "rw" })
}
