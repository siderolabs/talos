// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ContainerDependencyStatusType is type of ContainerDependencyStatus resource.
const ContainerDependencyStatusType = resource.Type("ContainerDependencyStatuses.containers.talos.dev")

// ContainerDependencyStatus resource reports the status of the container's readiness gates.
type ContainerDependencyStatus = typed.Resource[ContainerDependencyStatusSpec, ContainerDependencyStatusExtension]

// ContainerDependencyStatusSpec is the spec for ContainerDependencyStatus.
//
//gotagsrewrite:gen
type ContainerDependencyStatusSpec struct {
	// WaitingFor lists the unmet readiness gates (image, mounts, dependsOn entries).
	WaitingFor []string `yaml:"waitingFor,omitempty" protobuf:"1"`
}

// NewContainerDependencyStatus initializes a ContainerDependencyStatus resource.
func NewContainerDependencyStatus(namespace resource.Namespace, id resource.ID) *ContainerDependencyStatus {
	return typed.NewResource[ContainerDependencyStatusSpec, ContainerDependencyStatusExtension](
		resource.NewMetadata(namespace, ContainerDependencyStatusType, id, resource.VersionUndefined),
		ContainerDependencyStatusSpec{},
	)
}

// ContainerDependencyStatusExtension is auxiliary resource data for ContainerDependencyStatus.
type ContainerDependencyStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerDependencyStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerDependencyStatusType,
		Aliases:          []resource.Type{"containerdependencystatus", "containerdependencystatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Waiting For",
				JSONPath: `{.waitingFor}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ContainerDependencyStatusType, &ContainerDependencyStatus{})
	if err != nil {
		panic(err)
	}
}

// CurrentDependencyStatus returns the container dependency status.
// Returns nil if doesn't exist yet.
func CurrentDependencyStatus(ctx context.Context, reader controller.Reader, containerID string) (*ContainerDependencyStatus, error) {
	containerDependencyStatuses, err := safe.ReaderListAll[*ContainerDependencyStatus](ctx, reader,
		state.WithLabelQuery(resource.LabelEqual(ContainerSpecIdLabel, containerID)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list dependency statuses %q: %w", containerID, err)
	}

	// InstanceController publishes one verdict per container, so the first match is the only one.
	for containerDependencyStatus := range containerDependencyStatuses.All() {
		return containerDependencyStatus, nil
	}

	return nil, nil
}
