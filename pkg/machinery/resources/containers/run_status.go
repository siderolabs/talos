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
	// Restarts is the number of restarts since the execution hash last changed.
	Restarts uint32 `yaml:"restarts" protobuf:"3"`
	// BackoffStep is how many times the restart delay has doubled.
	BackoffStep uint32 `yaml:"backoffStep" protobuf:"4"`
	// RestartedGeneration is the generation whose restart has been charged, if Restarted.
	RestartedGeneration uint64 `yaml:"restartedGeneration,omitempty" protobuf:"5"`
	Restarted           bool   `yaml:"restarted,omitempty" protobuf:"6"`
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
