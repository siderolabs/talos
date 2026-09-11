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
	// Health is the coarse summary of State, kept stable across internal changes.
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
