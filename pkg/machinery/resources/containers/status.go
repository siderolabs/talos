// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ContainerStatusType is type of ContainerStatus resource.
const ContainerStatusType = resource.Type("ContainerStatuses.containers.talos.dev")

// ContainerStatus resource is the aggregated, user-facing status of a container.
//
// It is produced by StatusController from the spec, the image status and the newest instance
// status. The ID matches the owning ContainerSpec. It is stored in memory only and does not survive
// a reboot.
type ContainerStatus = typed.Resource[ContainerStatusSpec, ContainerStatusExtension]

// ContainerStatusSpec is the spec for ContainerStatus.
//
//gotagsrewrite:gen
type ContainerStatusSpec struct {
	// State is the fine-grained lifecycle position, derived from the newest instance.
	State ContainerState `yaml:"state" protobuf:"1"`
	// Health is the coarse user-facing status.
	Health ContainerHealth `yaml:"health" protobuf:"2"`
	// Image is the resolved digest once the pull completes, otherwise the requested reference.
	Image string `yaml:"image,omitempty" protobuf:"3"`
	// PID of the running task; zero when not running.
	PID uint32 `yaml:"pid,omitempty" protobuf:"4"`
	// ExitCode of the last task exit.
	ExitCode int32 `yaml:"exitCode,omitempty" protobuf:"5"`
	// RestartCount is the current instance generation, i.e. restarts beyond the first start.
	RestartCount uint64 `yaml:"restartCount" protobuf:"6"`
	// Error is the last failure, verbatim, from whichever stage produced it.
	Error string `yaml:"error,omitempty" protobuf:"7"`
	// WaitingFor lists the unmet readiness gates (image, mounts, dependsOn entries) while State is pending.
	WaitingFor []string `yaml:"waitingFor,omitempty" protobuf:"8"`
}

// NewContainerStatus initializes a ContainerStatus resource.
func NewContainerStatus(namespace resource.Namespace, id resource.ID) *ContainerStatus {
	return typed.NewResource[ContainerStatusSpec, ContainerStatusExtension](
		resource.NewMetadata(namespace, ContainerStatusType, id, resource.VersionUndefined),
		ContainerStatusSpec{},
	)
}

// ContainerStatusExtension is auxiliary resource data for ContainerStatus.
type ContainerStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerStatusType,
		Aliases:          []resource.Type{"containerstatus", "containerstatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "State",
				JSONPath: `{.state}`,
			},
			{
				Name:     "Health",
				JSONPath: `{.health}`,
			},
			{
				Name:     "Restarts",
				JSONPath: `{.restartCount}`,
			},
			{
				Name:     "Image",
				JSONPath: `{.image}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(ContainerStatusType, &ContainerStatus{}); err != nil {
		panic(err)
	}
}

// Update derives the aggregated status for one container from its already-resolved inputs.
//
// Most of it is recomputed from scratch, but the few values whose source disappears from under them
// — Health while an instance is on its way out, and the last execution's outcome between instances —
// carry over from the value this one replaces.
func (containerStatusSpec *ContainerStatusSpec) Update(
	containerSpec *ContainerSpec,
	containerImageStatus *ContainerImageStatus,
	containerInstanceStatus *ContainerInstanceStatus,
	containerInstanceSpec *ContainerInstanceSpec,
	imageDigest string,
	waitingFor []string,
	restartInterval time.Duration,
) {
	prev := *containerStatusSpec

	// The instance spec is destroyed as soon as RuntimeController releases its finalizer, one or more
	// passes before the instance status it wrote is cleaned up. A status with no spec behind it is an
	// instance that is already gone, not one that is still running.
	instanceGone := containerInstanceStatus != nil && containerInstanceSpec == nil

	isStopping := instanceGone ||
		containerInstanceSpec != nil && containerInstanceSpec.Metadata().Phase() == resource.PhaseTearingDown

	containerStatusSpec.Image = ResolveReportedImage(containerSpec, containerInstanceSpec, imageDigest)

	containerStatusSpec.PID = 0
	containerStatusSpec.WaitingFor = nil

	// There is no instance status between generations, nor for as long as a restart waits on a gate.
	// The last execution's outcome is what an operator is looking at in exactly those windows, so it
	// survives the gap rather than reading back as a container that never crashed.
	containerStatusSpec.RestartCount = prev.RestartCount
	containerStatusSpec.ExitCode = prev.ExitCode

	if containerInstanceStatus != nil {
		containerStatusSpec.RestartCount = containerInstanceStatus.TypedSpec().Generation

		containerStatusSpec.ExitCode = containerInstanceStatus.TypedSpec().ExitCode
		// The finalizer is only released once the task has been removed, so a PID reported by an
		// instance whose spec is already gone belongs to a task that is not there any more.
		if containerInstanceStatus.TypedSpec().Phase == ContainerInstancePhaseRunning && !instanceGone {
			containerStatusSpec.PID = containerInstanceStatus.TypedSpec().PID
		}
	}

	containerStatusSpec.State = ResolveContainerState(containerInstanceStatus, containerImageStatus, len(waitingFor) == 0, isStopping, restartInterval)

	containerStatusSpec.Health = containerStatusSpec.State.Health(prev.Health)

	if containerStatusSpec.State == ContainerStatePending {
		containerStatusSpec.WaitingFor = waitingFor
	}

	containerStatusSpec.Error = DeriveReportedError(containerInstanceStatus, containerImageStatus)

	// Same reasoning as RestartCount and ExitCode: keep the reason the last execution ended visible
	// while the status that reported it is gone.
	if containerStatusSpec.Error == "" && containerInstanceStatus == nil {
		containerStatusSpec.Error = prev.Error
	}
}

// ResolveReportedImage picks the image to report.
func ResolveReportedImage(containerSpec *ContainerSpec, containerInstanceSpec *ContainerInstanceSpec, imageDigest string) string {
	if containerInstanceSpec != nil && containerInstanceSpec.TypedSpec().Image != "" {
		return containerInstanceSpec.TypedSpec().Image
	}

	if imageDigest != "" {
		return imageDigest
	}

	return containerSpec.TypedSpec().Image.Ref
}

// ResolveContainerState maps the observable resources onto a container state.
//
// There is no terminal state: a finished instance means a restart is pending, which is exited while
// it is still fresh and backoff once restartInterval has elapsed waiting for it.
func ResolveContainerState(
	containerInstanceStatus *ContainerInstanceStatus,
	containerImageStatus *ContainerImageStatus,
	gatesReady bool,
	isStopping bool,
	restartInterval time.Duration,
) ContainerState {
	if containerInstanceStatus != nil {
		if isStopping {
			return ContainerStateStopping
		}

		return resolveInstanceState(containerInstanceStatus, restartInterval)
	}

	// Only the phases that say something the gate check cannot: an image still coming down, or one
	// that will not. Pending and Ready are left to the gate check below, which reports "image" as
	// unmet for as long as there is no usable digest.
	if containerImageStatus != nil {
		switch containerImageStatus.TypedSpec().Phase {
		case ContainerImagePhasePulling:
			return ContainerStatePulling
		case ContainerImagePhaseFailed:
			return ContainerStateBackoff
		case ContainerImagePhasePending, ContainerImagePhaseReady:
		}
	}

	if !gatesReady {
		return ContainerStatePending
	}

	return ContainerStateStarting
}

func resolveInstanceState(containerInstanceStatus *ContainerInstanceStatus, restartInterval time.Duration) ContainerState {
	switch containerInstanceStatus.TypedSpec().Phase {
	case ContainerInstancePhaseCreated:
		return ContainerStateStarting
	case ContainerInstancePhaseRunning:
		return ContainerStateRunning
	case ContainerInstancePhaseTerminated, ContainerInstancePhaseFailed:
		if _, stillInWindow := containerInstanceStatus.TypedSpec().RestartWindowWakeAfter(restartInterval).Get(); stillInWindow {
			return ContainerStateExited
		}

		return ContainerStateBackoff
	}

	return ContainerStateStarting
}
