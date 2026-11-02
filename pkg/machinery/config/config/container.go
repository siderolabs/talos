// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"time"

	"github.com/siderolabs/gen/optional"
)

// ContainerConfig defines the interface to access container configuration.
//
//nolint:interfacebloat
type ContainerConfig interface {
	NamedDocument

	// Marker for findMatchingDocs[T]
	ContainerConfigSignal()

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
	// Devices to hand to the container.
	Devices() []ContainerDeviceConfig
	// Security settings; never nil.
	Security() ContainerSecurityConfig
	// Network settings; never nil.
	Network() ContainerNetworkConfig
	// Resources requests and limits; never nil.
	Resources() ContainerResourcesConfig
	// DependsOn conditions gating startup; never nil.
	DependsOn() ContainerDependsOnConfig
	// HealthCheck probing the running container; None means health follows the lifecycle state.
	HealthCheck() optional.Optional[ContainerHealthCheckConfig]
	// StopTimeout is the time between SIGTERM and SIGKILL.
	StopTimeout() time.Duration
	// Restart settings; never nil.
	Restart() ContainerRestartConfig
}

// ContainerMountConfig defines a single container mount.
//
// Exactly one of the three sources is present.
type ContainerMountConfig interface {
	Volume() optional.Optional[ContainerVolumeMountConfig]
	Tmpfs() optional.Optional[ContainerTmpfsMountConfig]
	HostPath() optional.Optional[ContainerHostPathMountConfig]
}

// ContainerVolumeMountConfig mounts a volume by name.
type ContainerVolumeMountConfig interface {
	// Name of the UserVolumeConfig, ExistingVolumeConfig or ExternalVolumeConfig document.
	//
	// Volume names are unique across those kinds, so the name resolves to one volume.
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

// ContainerDeviceConfig defines a single device handed to a container.
//
// Exactly one source is present.
type ContainerDeviceConfig interface {
	RawVolume() optional.Optional[ContainerRawVolumeDeviceConfig]
}

// ContainerRawVolumeDeviceConfig hands a raw volume to a container as a block device.
type ContainerRawVolumeDeviceConfig interface {
	// Name of the RawVolumeConfig document.
	Name() string
	// Destination of the device node inside the container.
	Destination() string
}

// ContainerSecurityProfile selects the security posture for a container.
type ContainerSecurityProfile string

// Container security profiles.
const (
	// ContainerSecurityProfileRestricted drops all capabilities, allows no devices and mounts
	// the rootfs and sysfs read-only. This is the default.
	ContainerSecurityProfileRestricted ContainerSecurityProfile = "restricted"
	// ContainerSecurityProfilePrivileged grants all grantable capabilities and all devices, and
	// mounts sysfs read-write.
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
	// SeccompProfile is the seccomp filter applied to the container; defaults to default.
	SeccompProfile() ContainerSeccompProfile
	// SeccompLocalhostProfile names the seccomp profile applied with the localhost seccomp
	// profile; empty otherwise.
	SeccompLocalhostProfile() string
}

// ContainerSeccompProfile selects the seccomp filter applied to a container.
type ContainerSeccompProfile string

// Container seccomp profiles.
const (
	// ContainerSeccompProfileDefault applies the containerd default seccomp profile. This is the
	// default.
	ContainerSeccompProfileDefault ContainerSeccompProfile = "default"
	// ContainerSeccompProfileUnconfined applies no seccomp filter at all.
	ContainerSeccompProfileUnconfined ContainerSeccompProfile = "unconfined"
	// ContainerSeccompProfileLocalhost applies a seccomp profile declared in the machine configuration.
	ContainerSeccompProfileLocalhost ContainerSeccompProfile = "localhost"
)

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
	// SwapLimit maps onto cgroup v2 memory.swap.max; None means unlimited, zero means no swap.
	SwapLimit() optional.Optional[uint64]
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
	// Network readiness conditions, e.g. addresses, connectivity.
	Network() []string
	// Time is true if the clock must be synchronized.
	Time() bool
	// Containers which must be running first, by document name.
	Containers() []string
}

// ContainerHealthCheckKind selects the health check probe type.
type ContainerHealthCheckKind string

// Container health check kinds.
const (
	// ContainerHealthCheckKindTCP opens a TCP connection.
	ContainerHealthCheckKindTCP ContainerHealthCheckKind = "tcp"
	// ContainerHealthCheckKindHTTP sends an HTTP GET request.
	ContainerHealthCheckKindHTTP ContainerHealthCheckKind = "http"
	// ContainerHealthCheckKindGRPC calls the gRPC health checking protocol.
	ContainerHealthCheckKindGRPC ContainerHealthCheckKind = "grpc"
)

// ContainerHealthCheckConfig defines a probe deciding whether a running container is healthy.
//
// The probe always targets the container's loopback address.
type ContainerHealthCheckConfig interface {
	// Kind of the probe.
	Kind() ContainerHealthCheckKind
	// Port to probe.
	Port() uint16
	// HTTPPath is the request path for an HTTP probe, defaulting to "/".
	HTTPPath() string
	// GRPCService is the service name for a gRPC probe; empty means the whole server.
	GRPCService() string
	// InitialDelay before the first check.
	InitialDelay() time.Duration
	// Interval between checks, with the default applied.
	Interval() time.Duration
	// Timeout of a single check, with the default applied.
	Timeout() time.Duration
	// FailureThreshold is the number of consecutive failures to report unhealthy, with the default applied.
	FailureThreshold() uint32
}

// ContainerRestartPolicy selects when a container is restarted after it exits.
type ContainerRestartPolicy string

// Container restart policies.
const (
	// ContainerRestartPolicyAlways restarts the container after every exit. This is the default.
	ContainerRestartPolicyAlways ContainerRestartPolicy = "always"
	// ContainerRestartPolicyOnFailure restarts the container after it failed.
	ContainerRestartPolicyOnFailure ContainerRestartPolicy = "on-failure"
	// ContainerRestartPolicyNever runs the container once.
	ContainerRestartPolicyNever ContainerRestartPolicy = "never"
)

// ContainerRestartConfig defines how a container is restarted after it exits.
type ContainerRestartConfig interface {
	// Policy is when the container is restarted; defaults to always.
	Policy() ContainerRestartPolicy
	// MaxRetries is how many times a failed container is restarted at most; None means unlimited.
	MaxRetries() optional.Optional[int]
	// Delay is the time to wait before a restart.
	Delay() time.Duration
	// MaxDelay is the longest time the delay grows to.
	MaxDelay() time.Duration
}
