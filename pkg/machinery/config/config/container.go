// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/siderolabs/gen/optional"

// ContainerConfig defines the interface to access container configuration.
//
//nolint:interfacebloat
type ContainerConfig interface {
	NamedDocument

	// Image is the OCI reference in canonical form.
	Image() string
	// Entrypoint overrides the image ENTRYPOINT; nil means use the image.
	Entrypoint() []string
	// Args overrides the image CMD; nil means use the image.
	Args() []string
	// WorkingDir overrides the image WORKDIR; empty means use the image.
	WorkingDir() string
	// RunAs settings; never nil.
	RunAs() ContainerRunAsConfig
	// Environment is merged over the image ENV.
	Environment() []string
	// Mounts to set up in the container's mount namespace.
	Mounts() []ContainerMountConfig
	// Security settings; never nil.
	Security() ContainerSecurityConfig
	// Network settings; never nil.
	Network() ContainerNetworkConfig
	// Resources requests and limits; never nil.
	Resources() ContainerResourcesConfig
	// DependsOn conditions gating startup; never nil.
	DependsOn() ContainerDependsOnConfig
}

// ContainerMountConfig defines a single container mount.
//
// Exactly one of the three sources is present.
type ContainerMountConfig interface {
	UserVolume() optional.Optional[ContainerUserVolumeMountConfig]
	Tmpfs() optional.Optional[ContainerTmpfsMountConfig]
	HostPath() optional.Optional[ContainerHostPathMountConfig]
}

// ContainerUserVolumeMountConfig mounts a user volume by name.
type ContainerUserVolumeMountConfig interface {
	// Name of the UserVolumeConfig document.
	Name() string
	// Destination inside the container.
	Destination() string
	// MountOptions with the writable default already applied.
	MountOptions() []string
}

// ContainerTmpfsMountConfig mounts a tmpfs.
type ContainerTmpfsMountConfig interface {
	// Destination inside the container.
	Destination() string
	// Size of the tmpfs; empty means the kernel default.
	Size() string
	// MountOptions with the writable default already applied.
	MountOptions() []string
}

// ContainerHostPathMountConfig bind-mounts a host path.
type ContainerHostPathMountConfig interface {
	// Source on the host.
	Source() string
	// Destination inside the container.
	Destination() string
	// MountOptions with the writable default already applied.
	MountOptions() []string
}

// ContainerSecurityProfile selects the security posture for a container.
type ContainerSecurityProfile string

// Container security profiles.
const (
	// ContainerSecurityProfileRestricted drops all capabilities, allows no devices and mounts
	// the rootfs and sysfs read-only. This is the default.
	ContainerSecurityProfileRestricted ContainerSecurityProfile = "restricted"
	// ContainerSecurityProfilePrivileged grants all grantable capabilities and all devices,
	// matching what extension services get implicitly.
	ContainerSecurityProfilePrivileged ContainerSecurityProfile = "privileged"
)

// ContainerSecurityConfig defines the container security settings.
type ContainerSecurityConfig interface {
	// Profile is the security posture; defaults to restricted.
	Profile() ContainerSecurityProfile
	// CapabilitiesAdd lists capabilities to grant on top of the profile.
	CapabilitiesAdd() []string
	// CapabilitiesDrop lists capabilities to remove; "ALL" is accepted.
	CapabilitiesDrop() []string
	// MachinedAccess publishes the container's PID for machined's API to recognize, and mounts
	// the machined API socket into the container.
	MachinedAccess() bool
}

// ContainerNetworkMode selects the container's network namespace.
type ContainerNetworkMode string

// Container network modes.
const (
	// ContainerNetworkModeNone gives the container its own empty network namespace. Default.
	ContainerNetworkModeNone ContainerNetworkMode = "none"
	// ContainerNetworkModeHost shares the host network namespace.
	ContainerNetworkModeHost ContainerNetworkMode = "host"
)

// ContainerNetworkConfig defines the container network settings.
type ContainerNetworkConfig interface {
	// Mode is the network namespace mode; defaults to none.
	Mode() ContainerNetworkMode
}

// ContainerResourcesConfig defines container resource limits.
type ContainerResourcesConfig interface {
	// MemoryLimit maps onto cgroup v2 memory.max; None means unlimited.
	MemoryLimit() optional.Optional[uint64]
	// CPULimit maps onto cgroup v2 cpu.max, in millicores; None means unlimited.
	CPULimit() optional.Optional[uint64]
}

// ContainerRunAsConfig defines the uid/gid to run the container's entrypoint as.
type ContainerRunAsConfig interface {
	// UID overrides the image USER's uid; None means use the image.
	UID() optional.Optional[int32]
	// GID overrides the image USER's gid; None means use the image.
	GID() optional.Optional[int32]
}

// ContainerDependsOnConfig defines the conditions gating container startup.
type ContainerDependsOnConfig interface {
	// Paths which must exist on the host.
	Paths() []string
	// Networks readiness conditions, e.g. addresses, connectivity.
	Networks() []string
	// Time is true if the clock must be synchronized.
	Time() bool
	// Containers which must be running first, by document name.
	Containers() []string
}
