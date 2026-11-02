// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ContainerRunStatusType is type of ContainerRunStatus resource.
const ContainerRunStatusType = resource.Type("ContainerRunStatuses.containers.talos.dev")

// ContainerRunStatus resource keeps the restart accounting of a container across its instances.
//
// The ID matches the owning ContainerSpec. It is stored in memory only, so a reboot or a restart of
// machined starts over.
type ContainerRunStatus = typed.Resource[ContainerRunStatusSpec, ContainerRunStatusExtension]

// ContainerRunStatusSpec is the spec for ContainerRunStatus.
//
//gotagsrewrite:gen
type ContainerRunStatusSpec struct {
	// ExecutionHash is the ContainerSpec execution hash the counters are for.
	ExecutionHash string `yaml:"executionHash,omitempty" protobuf:"1"`
	// NextGeneration is the generation of the next instance.
	NextGeneration uint64 `yaml:"nextGeneration" protobuf:"2"`
	// Restarts is the number of restarts since the execution hash or the restart policy last changed.
	Restarts uint32 `yaml:"restarts" protobuf:"3"`
	// BackoffStep is how many times the restart delay has doubled.
	BackoffStep uint32 `yaml:"backoffStep" protobuf:"4"`
	// RestartedGeneration is the generation whose restart has been charged, if Restarted.
	RestartedGeneration uint64 `yaml:"restartedGeneration,omitempty" protobuf:"5"`
	Restarted           bool   `yaml:"restarted,omitempty" protobuf:"6"`
	// Policy is the restart policy the restarts were counted under.
	Policy ContainerRestartPolicy `yaml:"policy,omitempty" protobuf:"7"`
	// FinishedGeneration is the generation of the finished instance FinishedState is the outcome of.
	FinishedGeneration uint64 `yaml:"finishedGeneration,omitempty" protobuf:"8"`
	// FinishedState is what InstanceController decided for that instance: exited while it is
	// restarted, completed or failed when it is not. It is published so that what reports the state
	// of the container doesn't decide it again.
	FinishedState ContainerState `yaml:"finishedState,omitempty" protobuf:"9"`
}

// NewContainerRunStatus initializes a ContainerRunStatus resource.
func NewContainerRunStatus(namespace resource.Namespace, id resource.ID) *ContainerRunStatus {
	return typed.NewResource[ContainerRunStatusSpec, ContainerRunStatusExtension](
		resource.NewMetadata(namespace, ContainerRunStatusType, id, resource.VersionUndefined),
		ContainerRunStatusSpec{},
	)
}

// ContainerRunStatusExtension is auxiliary resource data for ContainerRunStatus.
type ContainerRunStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerRunStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerRunStatusType,
		Aliases:          []resource.Type{"containerrunstatus", "containerrunstatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Restarts",
				JSONPath: `{.restarts}`,
			},
			{
				Name:     "Next Generation",
				JSONPath: `{.nextGeneration}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(ContainerRunStatusType, &ContainerRunStatus{}); err != nil {
		panic(err)
	}
}

// RestartCharged reports whether the restart of the instance of the generation has been charged.
func (runStatus ContainerRunStatusSpec) RestartCharged(generation uint64) bool {
	return runStatus.Restarted && runStatus.RestartedGeneration == generation
}

// For returns the run status as counted for the container spec.
//
// A different execution configuration starts the restarts, the backoff and the outcome over. A
// different restart policy starts the restarts over, so that restarts made under one policy don't
// count against the maxRetries of another; the backoff and the charged restart are kept, as the
// container is the same.
func (runStatus ContainerRunStatusSpec) For(executionHash string, policy ContainerRestartPolicy) ContainerRunStatusSpec {
	if runStatus.ExecutionHash != executionHash {
		return ContainerRunStatusSpec{
			ExecutionHash:  executionHash,
			NextGeneration: runStatus.NextGeneration,
			Policy:         policy,
		}
	}

	if runStatus.Policy != policy {
		runStatus.Restarts = 0
		runStatus.Policy = policy
	}

	return runStatus
}

// Outcome returns the state InstanceController decided for the finished instance of the generation,
// false if it hasn't decided it (yet).
func (runStatus ContainerRunStatusSpec) Outcome(generation uint64) (ContainerState, bool) {
	if runStatus.FinishedGeneration != generation || runStatus.FinishedState == ContainerStatePending {
		return ContainerStatePending, false
	}

	return runStatus.FinishedState, true
}

// Replaced reports whether the instance runs a configuration other than the one counted for, i.e.
// it is to be replaced rather than restarted.
func (runStatus ContainerRunStatusSpec) Replaced(instanceSpec ContainerInstanceSpecSpec) bool {
	return instanceSpec.ExecutionHash != runStatus.ExecutionHash
}
