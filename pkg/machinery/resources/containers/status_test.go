// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestResolveContainerState(t *testing.T) {
	t.Parallel()

	const restartInterval = 5 * time.Second

	newInstanceStatus := func(phase containers.ContainerInstancePhase, finishedAt time.Time) *containers.ContainerInstanceStatus {
		status := containers.NewContainerInstanceStatus(containers.NamespaceName, "nginx-0")
		status.TypedSpec().Phase = phase
		status.TypedSpec().FinishedAt = finishedAt

		return status
	}

	newImageStatus := func(phase containers.ContainerImagePhase) *containers.ContainerImageStatus {
		status := containers.NewContainerImageStatus(containers.NamespaceName, "nginx")
		status.TypedSpec().Phase = phase

		return status
	}

	tests := []struct {
		name                    string
		containerInstanceStatus *containers.ContainerInstanceStatus
		containerImageStatus    *containers.ContainerImageStatus
		gatesReady              bool
		isStopping              bool
		want                    containers.ContainerState
	}{
		{
			name: "no instance, no image, gates unmet",
			want: containers.ContainerStatePending,
		},
		{
			name:       "no instance, no image, gates met",
			gatesReady: true,
			want:       containers.ContainerStateStarting,
		},
		{
			name:                 "no instance, image pulling",
			containerImageStatus: newImageStatus(containers.ContainerImagePhasePulling),
			gatesReady:           true,
			want:                 containers.ContainerStatePulling,
		},
		{
			name:                 "no instance, image failed",
			containerImageStatus: newImageStatus(containers.ContainerImagePhaseFailed),
			gatesReady:           true,
			want:                 containers.ContainerStateBackoff,
		},
		{
			name:                 "no instance, image pending, gates unmet",
			containerImageStatus: newImageStatus(containers.ContainerImagePhasePending),
			want:                 containers.ContainerStatePending,
		},
		{
			name:                 "no instance, image ready, gates met",
			containerImageStatus: newImageStatus(containers.ContainerImagePhaseReady),
			gatesReady:           true,
			want:                 containers.ContainerStateStarting,
		},
		{
			name:                    "isStopping wins over instance phase",
			containerInstanceStatus: newInstanceStatus(containers.ContainerInstancePhaseRunning, time.Time{}),
			isStopping:              true,
			want:                    containers.ContainerStateStopping,
		},
		{
			name:                    "instance created",
			containerInstanceStatus: newInstanceStatus(containers.ContainerInstancePhaseCreated, time.Time{}),
			want:                    containers.ContainerStateStarting,
		},
		{
			name:                    "instance running",
			containerInstanceStatus: newInstanceStatus(containers.ContainerInstancePhaseRunning, time.Time{}),
			want:                    containers.ContainerStateRunning,
		},
		{
			name:                    "instance terminated, still inside restart window",
			containerInstanceStatus: newInstanceStatus(containers.ContainerInstancePhaseTerminated, time.Now()),
			want:                    containers.ContainerStateExited,
		},
		{
			name:                    "instance terminated, restart window elapsed",
			containerInstanceStatus: newInstanceStatus(containers.ContainerInstancePhaseTerminated, time.Now().Add(-2*restartInterval)),
			want:                    containers.ContainerStateBackoff,
		},
		{
			name:                    "instance failed, still inside restart window",
			containerInstanceStatus: newInstanceStatus(containers.ContainerInstancePhaseFailed, time.Now()),
			want:                    containers.ContainerStateExited,
		},
		{
			name:                    "instance failed, restart window elapsed",
			containerInstanceStatus: newInstanceStatus(containers.ContainerInstancePhaseFailed, time.Now().Add(-2*restartInterval)),
			want:                    containers.ContainerStateBackoff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.ResolveContainerState(tt.containerInstanceStatus, tt.containerImageStatus, tt.gatesReady, tt.isStopping, restartInterval)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveReportedImage(t *testing.T) {
	t.Parallel()

	const ref = "docker.io/library/nginx:latest"

	newContainerSpec := func() *containers.ContainerSpec {
		spec := containers.NewContainerSpec(containers.NamespaceName, "nginx")
		spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: ref}

		return spec
	}

	newInstanceSpec := func(image string) *containers.ContainerInstanceSpec {
		spec := containers.NewContainerInstanceSpec(containers.NamespaceName, "nginx-0")
		spec.TypedSpec().Image = image

		return spec
	}

	tests := []struct {
		name                  string
		containerInstanceSpec *containers.ContainerInstanceSpec
		imageDigest           string
		want                  string
	}{
		{
			name: "no instance, no digest: falls back to the reference",
			want: ref,
		},
		{
			name:        "no instance, digest resolved",
			imageDigest: "sha256:new",
			want:        "sha256:new",
		},
		{
			name:                  "instance image wins over the resolved digest",
			containerInstanceSpec: newInstanceSpec("sha256:old"),
			imageDigest:           "sha256:new",
			want:                  "sha256:old",
		},
		{
			name:                  "instance present without an image falls through to the digest",
			containerInstanceSpec: newInstanceSpec(""),
			imageDigest:           "sha256:new",
			want:                  "sha256:new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := containers.ResolveReportedImage(newContainerSpec(), tt.containerInstanceSpec, tt.imageDigest)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContainerStatusSpecUpdate(t *testing.T) {
	t.Parallel()

	const (
		imageRef        = "docker.io/library/nginx:latest"
		digest          = "sha256:new"
		restartInterval = 5 * time.Second
	)

	newContainerSpec := func() *containers.ContainerSpec {
		spec := containers.NewContainerSpec(containers.NamespaceName, "nginx")
		spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: imageRef}

		return spec
	}

	newInstanceStatus := func(mutate func(*containers.ContainerInstanceStatusSpec)) *containers.ContainerInstanceStatus {
		status := containers.NewContainerInstanceStatus(containers.NamespaceName, "nginx-0")
		mutate(status.TypedSpec())

		return status
	}

	newImageStatus := func(phase containers.ContainerImagePhase, errorMessage string) *containers.ContainerImageStatus {
		status := containers.NewContainerImageStatus(containers.NamespaceName, "nginx")
		status.TypedSpec().Phase = phase
		status.TypedSpec().Error = errorMessage

		return status
	}

	newInstanceSpec := func(image string, phase resource.Phase) *containers.ContainerInstanceSpec {
		spec := containers.NewContainerInstanceSpec(containers.NamespaceName, "nginx-0")
		spec.TypedSpec().Image = image
		spec.Metadata().SetPhase(phase)

		return spec
	}

	tests := []struct {
		name                    string
		prev                    containers.ContainerStatusSpec
		containerImageStatus    *containers.ContainerImageStatus
		containerInstanceStatus *containers.ContainerInstanceStatus
		containerInstanceSpec   *containers.ContainerInstanceSpec
		imageDigest             string
		waitingFor              []string
		want                    containers.ContainerStatusSpec
	}{
		{
			name:       "fresh container waiting on its gates",
			waitingFor: []string{"image"},
			want: containers.ContainerStatusSpec{
				State:      containers.ContainerStatePending,
				Health:     containers.ContainerHealthPending,
				Image:      imageRef,
				WaitingFor: []string{"image"},
			},
		},
		{
			name: "running instance is projected onto the aggregate",
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseRunning
				spec.Generation = 2
				spec.PID = 42
			}),
			containerInstanceSpec: newInstanceSpec("", resource.PhaseRunning),
			imageDigest:           digest,
			want: containers.ContainerStatusSpec{
				State:        containers.ContainerStateRunning,
				Health:       containers.ContainerHealthHealthy,
				Image:        digest,
				PID:          42,
				RestartCount: 2,
			},
		},
		{
			name: "PID is dropped for an instance that is no longer running",
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseTerminated
				spec.Generation = 1
				spec.PID = 42
				spec.ExitCode = 1
				spec.FinishedAt = time.Now()
			}),
			containerInstanceSpec: newInstanceSpec("", resource.PhaseRunning),
			want: containers.ContainerStatusSpec{
				State:        containers.ContainerStateExited,
				Health:       containers.ContainerHealthDegraded,
				Image:        imageRef,
				ExitCode:     1,
				RestartCount: 1,
			},
		},
		{
			name: "waitingFor is dropped once the state is no longer pending",
			prev: containers.ContainerStatusSpec{
				State:      containers.ContainerStatePending,
				Health:     containers.ContainerHealthPending,
				WaitingFor: []string{"image"},
			},
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseRunning
				spec.PID = 7
			}),
			containerInstanceSpec: newInstanceSpec("", resource.PhaseRunning),
			waitingFor:            []string{"mounts"},
			want: containers.ContainerStatusSpec{
				State:  containers.ContainerStateRunning,
				Health: containers.ContainerHealthHealthy,
				Image:  imageRef,
				PID:    7,
			},
		},
		{
			name: "the last execution's outcome survives the gap between instances",
			prev: containers.ContainerStatusSpec{
				State:        containers.ContainerStateBackoff,
				Health:       containers.ContainerHealthDegraded,
				PID:          9,
				RestartCount: 3,
				ExitCode:     7,
				Error:        "task exited",
			},
			want: containers.ContainerStatusSpec{
				State:        containers.ContainerStateStarting,
				Health:       containers.ContainerHealthPulling,
				Image:        imageRef,
				RestartCount: 3,
				ExitCode:     7,
				Error:        "task exited",
			},
		},
		{
			name: "a present instance status overwrites the carried-over outcome",
			prev: containers.ContainerStatusSpec{
				RestartCount: 3,
				ExitCode:     7,
			},
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseRunning
				spec.Generation = 5
				spec.PID = 11
			}),
			containerInstanceSpec: newInstanceSpec("", resource.PhaseRunning),
			want: containers.ContainerStatusSpec{
				State:        containers.ContainerStateRunning,
				Health:       containers.ContainerHealthHealthy,
				Image:        imageRef,
				PID:          11,
				RestartCount: 5,
			},
		},
		{
			name: "a present instance status without an error clears the carried-over one",
			prev: containers.ContainerStatusSpec{
				Error: "task exited",
			},
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseRunning
				spec.PID = 11
			}),
			containerInstanceSpec: newInstanceSpec("", resource.PhaseRunning),
			want: containers.ContainerStatusSpec{
				State:  containers.ContainerStateRunning,
				Health: containers.ContainerHealthHealthy,
				Image:  imageRef,
				PID:    11,
			},
		},
		{
			name: "stopping carries the previous health of a healthy container",
			prev: containers.ContainerStatusSpec{
				State:  containers.ContainerStateRunning,
				Health: containers.ContainerHealthHealthy,
			},
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseRunning
				spec.PID = 11
			}),
			containerInstanceSpec: newInstanceSpec(digest, resource.PhaseTearingDown),
			want: containers.ContainerStatusSpec{
				State:  containers.ContainerStateStopping,
				Health: containers.ContainerHealthHealthy,
				Image:  digest,
				PID:    11,
			},
		},
		{
			name: "stopping carries the previous health of a degraded container",
			prev: containers.ContainerStatusSpec{
				State:  containers.ContainerStateBackoff,
				Health: containers.ContainerHealthDegraded,
			},
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseTerminated
				spec.ExitCode = 2
				spec.FinishedAt = time.Now()
			}),
			containerInstanceSpec: newInstanceSpec(digest, resource.PhaseTearingDown),
			want: containers.ContainerStatusSpec{
				State:    containers.ContainerStateStopping,
				Health:   containers.ContainerHealthDegraded,
				Image:    digest,
				ExitCode: 2,
			},
		},
		{
			name: "an instance whose spec is already destroyed is still stopping",
			prev: containers.ContainerStatusSpec{
				State:  containers.ContainerStateRunning,
				Health: containers.ContainerHealthHealthy,
				Image:  digest,
				PID:    1234,
			},
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseRunning
				spec.Generation = 1
				spec.PID = 1234
			}),
			imageDigest: digest,
			want: containers.ContainerStatusSpec{
				State:        containers.ContainerStateStopping,
				Health:       containers.ContainerHealthHealthy,
				Image:        digest,
				RestartCount: 1,
			},
		},
		{
			name: "a destroyed instance spec carries the previous health of a degraded container",
			prev: containers.ContainerStatusSpec{
				State:  containers.ContainerStateBackoff,
				Health: containers.ContainerHealthDegraded,
			},
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseTerminated
				spec.ExitCode = 2
				spec.FinishedAt = time.Now()
			}),
			imageDigest: digest,
			want: containers.ContainerStatusSpec{
				State:    containers.ContainerStateStopping,
				Health:   containers.ContainerHealthDegraded,
				Image:    digest,
				ExitCode: 2,
			},
		},
		{
			name:                 "a failed pull surfaces through the aggregate",
			containerImageStatus: newImageStatus(containers.ContainerImagePhaseFailed, "pull failed"),
			waitingFor:           []string{"image"},
			want: containers.ContainerStatusSpec{
				State:  containers.ContainerStateBackoff,
				Health: containers.ContainerHealthDegraded,
				Image:  imageRef,
				Error:  "pull failed",
			},
		},
		{
			name: "the running instance's image outranks a newly resolved digest",
			containerInstanceStatus: newInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
				spec.Phase = containers.ContainerInstancePhaseRunning
				spec.PID = 11
			}),
			containerInstanceSpec: newInstanceSpec("sha256:old", resource.PhaseRunning),
			imageDigest:           digest,
			want: containers.ContainerStatusSpec{
				State:  containers.ContainerStateRunning,
				Health: containers.ContainerHealthHealthy,
				Image:  "sha256:old",
				PID:    11,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.prev

			got.Update(
				newContainerSpec(),
				tt.containerImageStatus,
				tt.containerInstanceStatus,
				tt.containerInstanceSpec,
				tt.imageDigest,
				tt.waitingFor,
				restartInterval,
			)

			assert.Equal(t, tt.want, got)
		})
	}
}
