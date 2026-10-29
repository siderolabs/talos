// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	timeres "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

// pathPollInterval is how often to re-check dependsOn.paths entries.
//
// Paths are the one dependency with no COSI equivalent, so they have to be polled.
const pathPollInterval = time.Second

// containerDependencyStabilityWindow is how long a dependsOn.containers entry's dependency must have
// been Running before it counts as healthy for gating purposes.
const containerDependencyStabilityWindow = 5 * time.Second

// ContainerSpecType is type of ContainerSpec resource.
const ContainerSpecType = resource.Type("ContainerSpecs.containers.talos.dev")

// ContainerSpec resource holds the desired state of a single container.
//
// It is the long-lived unit: it exists for as long as its ContainerConfig document does, and
// outlives any number of executions. A single execution is a ContainerInstanceSpec.
type ContainerSpec = typed.Resource[ContainerSpecSpec, ContainerSpecExtension]

// ContainerSpecSpec is the spec for ContainerSpec.
//
//gotagsrewrite:gen
type ContainerSpecSpec struct {
	// Image is the OCI reference in canonical form, already normalized by ContainerConfigController.
	Image ContainerImageSpec `yaml:"image" protobuf:"1"`

	Entrypoint  []string           `yaml:"entrypoint,omitempty" protobuf:"2"`
	Args        []string           `yaml:"args,omitempty" protobuf:"3"`
	WorkingDir  string             `yaml:"workingDir,omitempty" protobuf:"4"`
	RunAs       ContainerRunAsSpec `yaml:"runAs,omitempty" protobuf:"5"`
	Environment []string           `yaml:"environment,omitempty" protobuf:"6"`

	Mounts []ContainerMountSpec `yaml:"mounts,omitempty" protobuf:"7"`

	Security  ContainerSecuritySpec  `yaml:"security,omitempty" protobuf:"8"`
	Network   ContainerNetworkSpec   `yaml:"network,omitempty" protobuf:"9"`
	Resources ContainerResourcesSpec `yaml:"resources,omitempty" protobuf:"10"`
	DependsOn ContainerDependsOnSpec `yaml:"dependsOn,omitempty" protobuf:"11"`

	Devices []ContainerDeviceSpec `yaml:"devices,omitempty" protobuf:"12"`

	StopTimeout time.Duration `yaml:"stopTimeout,omitempty" protobuf:"13"`
}

// Ready reports the container's unmet dependencies (image, mounts, devices, dependsOn gates),
// and how soon to recheck them.
//
// containerID is the owning ContainerSpec resource's ID: the spec itself doesn't carry it.
func (containerSpec ContainerSpecSpec) Ready(ctx context.Context, r controller.Reader, containerID string) ([]string, optional.Optional[time.Duration], error) {
	var waitingFor []string

	imageDigest, err := GetImageDigest(ctx, r, containerID, containerSpec.Image.Ref)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	if imageDigest == "" {
		waitingFor = append(waitingFor, "image")
	}

	resolved, err := GetResolvedMountStatus(ctx, r, containerID)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	waitingFor = append(waitingFor, containerSpec.ResolvedWaitingFor(resolved)...)

	unmet, wakeUpAfter, err := containerSpec.DependsOn.Ready(ctx, r)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	waitingFor = append(waitingFor, unmet...)

	return waitingFor, wakeUpAfter, nil
}

// CurrentImageStatus returns the image status describing the reference the container spec asks for,
// or nil if there is none.
//
// ImageController rewrites the status only on its own next reconcile, so right after a reference
// edit the one on record still describes the previous image: its phase and its error belong to a
// pull that is no longer the one being waited on. Same check GetImageDigest makes.
//
// containerID is the owning ContainerSpec resource's ID: the spec itself doesn't carry it.
func (containerSpec ContainerSpecSpec) CurrentImageStatus(
	ctx context.Context,
	r controller.Reader,
	containerID string,
) (*ContainerImageStatus, error) {
	containerImageStatus, err := safe.ReaderGetByID[*ContainerImageStatus](ctx, r, containerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get image status %q: %w", containerID, err)
	}

	if containerImageStatus.TypedSpec().Image != containerSpec.Image.Ref {
		return nil, nil
	}

	return containerImageStatus, nil
}

// GetResolvedMountStatus returns the mounts and devices MountController has most recently resolved
// for this container, or an empty status if it has not written one yet, or has not marked one ready.
//
// This does not check the result against the spec's own declared mounts and devices: the status is
// written by another controller, so a spec edit is visible here before the resolution catches up,
// and a caller that cares whether the result is stale must check it separately, e.g. with
// ResolvedMatchDeclared.
//
// containerID is the owning ContainerSpec resource's ID.
func GetResolvedMountStatus(
	ctx context.Context,
	r controller.Reader,
	containerID string,
) (ContainerMountStatusSpec, error) {
	status, err := safe.ReaderGetByID[*ContainerMountStatus](ctx, r, containerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return ContainerMountStatusSpec{}, nil
		}

		return ContainerMountStatusSpec{}, fmt.Errorf("failed to get mount status %q: %w", containerID, err)
	}

	if !status.TypedSpec().Ready {
		return ContainerMountStatusSpec{}, nil
	}

	return *status.TypedSpec(), nil
}

// ResolvedMatchDeclared reports whether resolved describes the mounts and devices the spec declares.
func (containerSpec ContainerSpecSpec) ResolvedMatchDeclared(resolved ContainerMountStatusSpec) bool {
	return len(containerSpec.ResolvedWaitingFor(resolved)) == 0
}

// ResolvedWaitingFor reports the gates ("mounts", "devices") resolved does not satisfy yet.
func (containerSpec ContainerSpecSpec) ResolvedWaitingFor(resolved ContainerMountStatusSpec) []string {
	var waitingFor []string

	if !MountsResolvedMatchDeclared(resolved.Mounts, containerSpec.Mounts) {
		waitingFor = append(waitingFor, "mounts")
	}

	if !DevicesResolvedMatchDeclared(resolved.Devices, containerSpec.Devices) {
		waitingFor = append(waitingFor, "devices")
	}

	return waitingFor
}

// InstanceProcessEqual compares the parts of the spec that describe the process itself, including how it is stopped.
func (containerSpec ContainerSpecSpec) InstanceProcessEqual(instanceSpec ContainerInstanceSpecSpec) bool {
	return slices.Equal(containerSpec.Entrypoint, instanceSpec.Entrypoint) &&
		slices.Equal(containerSpec.Args, instanceSpec.Args) &&
		containerSpec.WorkingDir == instanceSpec.WorkingDir &&
		containerSpec.RunAs.Equal(instanceSpec.RunAs) &&
		slices.Equal(containerSpec.Environment, instanceSpec.Environment) &&
		containerSpec.StopTimeout == instanceSpec.StopTimeout
}

// MountsResolvedMatchDeclared reports whether resolved describes the same mounts as declared.
//
// nolint: gocyclo
func MountsResolvedMatchDeclared(resolved []ResolvedMountSpec, declared []ContainerMountSpec) bool {
	if len(resolved) != len(declared) {
		return false
	}

	for i, mount := range declared {
		r := resolved[i]

		if r.Kind != mount.Kind || r.Destination != mount.Destination || r.Size != mount.Size {
			return false
		}

		if !slices.Equal(r.Options, mount.Options) {
			return false
		}

		if mount.Kind == MountKindHostPath && r.Source != mount.Source {
			return false
		}

		if mount.Kind == MountKindVolume && r.VolumeName != mount.VolumeName {
			return false
		}
	}

	return true
}

// DevicesResolvedMatchDeclared reports whether resolved describes the same devices as declared.
func DevicesResolvedMatchDeclared(resolved []ResolvedDeviceSpec, declared []ContainerDeviceSpec) bool {
	return slices.EqualFunc(resolved, declared, func(r ResolvedDeviceSpec, device ContainerDeviceSpec) bool {
		return r.Kind == device.Kind &&
			r.Destination == device.Destination &&
			r.VolumeName == device.VolumeName
	})
}

// ContainerMountSpec is a resolved mount.
//
// Exactly one of VolumeName, Tmpfs or HostPath describes the source; Kind says which.
//
//gotagsrewrite:gen
type ContainerMountSpec struct {
	// Kind is one of "volume", "tmpfs" or "hostPath".
	Kind string `yaml:"kind" protobuf:"1"`
	// Source is the host path for a hostPath mount.
	Source string `yaml:"source,omitempty" protobuf:"3"`
	// Destination inside the container.
	Destination string `yaml:"destination" protobuf:"4"`
	// Size of a tmpfs mount, in bytes; zero means the kernel default.
	Size uint64 `yaml:"size,omitempty" protobuf:"5"`
	// Options with the writable default already applied.
	Options []string `yaml:"options,omitempty" protobuf:"6"`
	// VolumeName is the name of the user, existing or external volume a volume mount refers to,
	// e.g. "web-content".
	VolumeName string `yaml:"volumeName,omitempty" protobuf:"7"`
}

// ReadOnly reports whether the mount asks for read-only access.
func (m ContainerMountSpec) ReadOnly() bool {
	return slices.Contains(m.Options, "ro")
}

// Mount kinds.
const (
	MountKindVolume   = "volume"
	MountKindTmpfs    = "tmpfs"
	MountKindHostPath = "hostPath"
)

// ContainerDeviceSpec is a device handed to the container.
//
//gotagsrewrite:gen
type ContainerDeviceSpec struct {
	// Kind is "rawVolume".
	Kind string `yaml:"kind" protobuf:"1"`
	// VolumeName is the name of the RawVolumeConfig a rawVolume device refers to, e.g. "data".
	VolumeName string `yaml:"volumeName,omitempty" protobuf:"2"`
	// Destination of the device node inside the container.
	Destination string `yaml:"destination" protobuf:"3"`
}

// Device kinds.
const (
	DeviceKindRawVolume = "rawVolume"
)

// ContainerSecuritySpec is the resolved security posture.
//
//gotagsrewrite:gen
type ContainerSecuritySpec struct {
	// Privileged grants all grantable capabilities and all devices, and mounts sysfs read-write.
	Privileged bool `yaml:"privileged,omitempty" protobuf:"1"`

	CapabilitiesAdd  []string `yaml:"capabilitiesAdd,omitempty" protobuf:"2"`
	CapabilitiesDrop []string `yaml:"capabilitiesDrop,omitempty" protobuf:"3"`

	// MachinedAccess publishes the container's PID as a ServicePID resource and mounts the
	// machined API socket into the container.
	MachinedAccess bool `yaml:"machinedAccess,omitempty" protobuf:"4"`
}

// Equal compares two security specs field by field, as they carry slices.
func (a ContainerSecuritySpec) Equal(b ContainerSecuritySpec) bool {
	return a.Privileged == b.Privileged &&
		slices.Equal(a.CapabilitiesAdd, b.CapabilitiesAdd) &&
		slices.Equal(a.CapabilitiesDrop, b.CapabilitiesDrop) &&
		a.MachinedAccess == b.MachinedAccess
}

// ContainerNetworkSpec is the resolved network configuration.
//
//gotagsrewrite:gen
type ContainerNetworkSpec struct {
	// HostNetwork shares the host network namespace instead of creating an empty one.
	HostNetwork bool `yaml:"hostNetwork,omitempty" protobuf:"1"`
}

// ContainerResourcesSpec is the resolved cgroup configuration, in bytes and millicores.
//
// Zero means unset, which for a limit means unlimited. The swap limit is a pointer because zero is
// meaningful there: it keeps the container out of swap, while nil leaves swap unlimited.
//
//gotagsrewrite:gen
type ContainerResourcesSpec struct {
	MemoryLimit uint64  `yaml:"memoryLimit,omitempty" protobuf:"1"`
	CPULimit    uint64  `yaml:"cpuLimit,omitempty" protobuf:"2"`
	SwapLimit   *uint64 `yaml:"swapLimit,omitempty" protobuf:"3"`
}

// Equal compares two resource specs by value: the swap limit is a pointer, which `==` would compare
// by identity, and an instance read back from the state never shares the spec's pointer.
func (a ContainerResourcesSpec) Equal(b ContainerResourcesSpec) bool {
	return a.MemoryLimit == b.MemoryLimit &&
		a.CPULimit == b.CPULimit &&
		pointerValuesEqual(a.SwapLimit, b.SwapLimit)
}

func pointerValuesEqual[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// ContainerDependsOnSpec is the resolved dependency set.
//
//gotagsrewrite:gen
type ContainerDependsOnSpec struct {
	Paths      []string `yaml:"paths,omitempty" protobuf:"1"`
	Networks   []string `yaml:"networks,omitempty" protobuf:"2"`
	Time       bool     `yaml:"time,omitempty" protobuf:"3"`
	Containers []string `yaml:"containers,omitempty" protobuf:"4"`
}

// Ready reports the declared dependsOn gates that are not yet satisfied, and how soon the caller
// should recheck the gates Ready cannot itself observe an event for: Paths, which are polled, and
// Containers, whose dependencies have to stay running for containerDependencyStabilityWindow.
//
// Returns: unsatisfied dependencies, duration to wait before rechecking, error.
func (dependsOn ContainerDependsOnSpec) Ready(
	ctx context.Context,
	r controller.Reader,
) ([]string, optional.Optional[time.Duration], error) {
	var waitingFor []string

	// dependsOn.network
	unmetNetwork, err := dependsOn.NetworkReady(ctx, r)
	if err != nil {
		return nil, optional.None[time.Duration](), fmt.Errorf("failed to check network ready: %w", err)
	}

	waitingFor = append(waitingFor, unmetNetwork...)

	// dependsOn.time
	timeReady, err := dependsOn.TimeReady(ctx, r)
	if err != nil {
		return nil, optional.None[time.Duration](), fmt.Errorf("failed to check time ready: %w", err)
	}

	if !timeReady {
		waitingFor = append(waitingFor, "time")
	}

	// dependsOn.paths
	for _, path := range dependsOn.Paths {
		if _, err := os.Stat(path); err != nil {
			waitingFor = append(waitingFor, "path: "+path)
		}
	}

	// dependsOn.containers
	unmetContainers, containersWakeUpAfter, err := dependsOn.ContainersReady(ctx, r)
	if err != nil {
		return nil, optional.None[time.Duration](), fmt.Errorf("failed to check container dependency readiness: %w", err)
	}

	waitingFor = append(waitingFor, unmetContainers...)

	var wakeUpAfter optional.Optional[time.Duration]
	if len(dependsOn.Paths) > 0 {
		// Paths have no event to wake us, so poll while any are declared.
		wakeUpAfter = optional.Some(pathPollInterval)
	}

	wakeUpAfter = EarliestWakeUpAfter(wakeUpAfter, containersWakeUpAfter)

	return waitingFor, wakeUpAfter, nil
}

// EarliestWakeUpAfter returns the smaller of two optional wake-up durations, treating an unset one as
// "no opinion" rather than as zero.
func EarliestWakeUpAfter(a, b optional.Optional[time.Duration]) optional.Optional[time.Duration] {
	av, aok := a.Get()
	bv, bok := b.Get()

	switch {
	case !aok:
		return b
	case !bok:
		return a
	case bv < av:
		return b
	default:
		return a
	}
}

// TimeReady reports whether the dependsOn.time gate is satisfied.
//
// A status resource that doesn't exist yet counts as not satisfied. If time sync is disabled on
// the node, this gate can never be satisfied, and a container declaring it stays blocked: the
// dependency was declared explicitly, so an unsynced clock should never be silently accepted.
func (dependsOn ContainerDependsOnSpec) TimeReady(ctx context.Context, r controller.Reader) (bool, error) {
	if !dependsOn.Time {
		// Doesn't depend on time sync, so it's satisfied regardless of the time status.
		return true, nil
	}

	status, err := safe.ReaderGetByID[*timeres.Status](ctx, r, timeres.StatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get time status: %w", err)
	}

	return status.TypedSpec().Synced, nil
}

// NetworkReady reports the declared dependsOn.network conditions that are not yet satisfied.
//
// A status resource that doesn't exist yet counts every declared condition as not satisfied.
func (dependsOn ContainerDependsOnSpec) NetworkReady(ctx context.Context, r controller.Reader) ([]string, error) {
	if dependsOn.Networks == nil {
		return nil, nil
	}

	status, err := safe.ReaderGetByID[*network.Status](ctx, r, network.StatusID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return nil, fmt.Errorf("failed to get network status: %w", err)
		}

		status = nil
	}

	var waitingFor []string

	for _, condition := range dependsOn.Networks {
		if !dependsOn.NetworkConditionMet(status, condition) {
			waitingFor = append(waitingFor, "network: "+condition)
		}
	}

	return waitingFor, nil
}

// ContainersReady reports the declared dependsOn.containers entries that are not yet trustworthy, and
// how soon to recheck one that is running but has not yet been running long enough to trust (see
// containerDependencyStabilityWindow).
//
// A container with no ContainerStatus yet counts as not satisfied, same as a network or time status
// that hasn't arrived: waiting is the correct answer, not an error. So does one that is stopping:
// it is on its way out, whatever its last health was.
func (dependsOn ContainerDependsOnSpec) ContainersReady(
	ctx context.Context, r controller.Reader,
) ([]string, optional.Optional[time.Duration], error) {
	var (
		waitingFor  []string
		wakeUpAfter optional.Optional[time.Duration]
	)

	for _, name := range dependsOn.Containers {
		status, err := safe.ReaderGetByID[*ContainerStatus](ctx, r, name)
		if err != nil {
			if state.IsNotFoundError(err) {
				waitingFor = append(waitingFor, "container: "+name)

				continue
			}

			return nil, optional.None[time.Duration](), fmt.Errorf("failed to get container status %q: %w", name, err)
		}

		// State as well as Health: an instance on its way out keeps the Health it had, by design, so
		// Health alone would let a dependency that is being torn down satisfy the gate.
		if status.TypedSpec().Health != ContainerHealthHealthy || status.TypedSpec().State != ContainerStateRunning {
			waitingFor = append(waitingFor, "container: "+name)

			continue
		}

		instanceStatus, err := LatestInstanceStatus(ctx, r, name)
		if err != nil {
			return nil, optional.None[time.Duration](), fmt.Errorf("failed to get instance status for container %q: %w", name, err)
		}

		// Health said Healthy but the newest instance status disagrees: a stale read between two
		// resources written by different controllers. Treat as not yet trustworthy rather than
		// erroring -- the next pass sees a consistent view.
		if instanceStatus == nil || instanceStatus.TypedSpec().Phase != ContainerInstancePhaseRunning {
			waitingFor = append(waitingFor, "container: "+name)

			continue
		}

		if since := time.Since(instanceStatus.TypedSpec().StartedAt); since < containerDependencyStabilityWindow {
			waitingFor = append(waitingFor, "container: "+name)
			wakeUpAfter = EarliestWakeUpAfter(wakeUpAfter, optional.Some(containerDependencyStabilityWindow-since))
		}
	}

	return waitingFor, wakeUpAfter, nil
}

// NetworkConditionMet reports whether one declared dependsOn.network condition is satisfied.
func (ContainerDependsOnSpec) NetworkConditionMet(status *network.Status, condition string) bool {
	if status == nil {
		return false
	}

	spec := status.TypedSpec()

	switch condition {
	case "addresses":
		return network.AddressReady(spec)
	case "connectivity":
		return network.ConnectivityReady(spec)
	case "hostname":
		return network.HostnameReady(spec)
	case "etcfiles":
		return network.EtcFilesReady(spec)
	default:
		// Validation rejects unknown conditions, so this is unreachable from configuration.
		return false
	}
}

// ContainerRunAsSpec is the resolved uid/gid override.
//
// Nil means use the image's own USER for that half.
//
//gotagsrewrite:gen
type ContainerRunAsSpec struct {
	UID *int32 `yaml:"uid,omitempty" protobuf:"1"`
	GID *int32 `yaml:"gid,omitempty" protobuf:"2"`
}

// Equal compares two RunAs specs, treating nil UID/GID halves as equal only to each other.
func (a ContainerRunAsSpec) Equal(b ContainerRunAsSpec) bool {
	return Int32PtrEqual(a.UID, b.UID) && Int32PtrEqual(a.GID, b.GID)
}

// ContainerImageSpec is a resolved container image reference.
//
//gotagsrewrite:gen
type ContainerImageSpec struct {
	Ref string `yaml:"ref,omitempty" protobuf:"1"`
}

// NewContainerSpec initializes a ContainerSpec resource.
func NewContainerSpec(namespace resource.Namespace, id resource.ID) *ContainerSpec {
	return typed.NewResource[ContainerSpecSpec, ContainerSpecExtension](
		resource.NewMetadata(namespace, ContainerSpecType, id, resource.VersionUndefined),
		ContainerSpecSpec{},
	)
}

// ContainerSpecExtension is auxiliary resource data for ContainerSpec.
type ContainerSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerSpecType,
		Aliases:          []resource.Type{"containerspec", "containerspecs"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Image",
				JSONPath: `{.image.ref}`,
			},
		},
		// the environment, the entrypoint and the arguments come from the machine configuration
		Sensitivity: meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ContainerSpecType, &ContainerSpec{})
	if err != nil {
		panic(err)
	}

	// the spec is sensitive, but it is safe to be read in the redacted form
	redactable.Register[ContainerSpecSpec, ContainerSpecExtension]()
}
