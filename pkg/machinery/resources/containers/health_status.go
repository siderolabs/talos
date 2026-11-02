// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ContainerHealthStatusType is type of ContainerHealthStatus resource.
const ContainerHealthStatusType = resource.Type("ContainerHealthStatuses.containers.talos.dev")

// ContainerHealthStatus resource reports the outcome of a container instance's health check.
//
// It is produced by HealthController, and only for an instance whose container has a health check
// configured. The ID matches the ContainerInstanceSpec it reports on, so the outcome of an earlier
// execution can never be mistaken for the current one. It is written once the outcome is known: no
// resource means no check has decided yet.
type ContainerHealthStatus = typed.Resource[ContainerHealthStatusSpec, ContainerHealthStatusExtension]

// ContainerHealthStatusSpec is the spec for ContainerHealthStatus.
//
//gotagsrewrite:gen
type ContainerHealthStatusSpec struct {
	// Healthy is true once a check has passed, and false once failureThreshold consecutive checks failed.
	Healthy bool `yaml:"healthy" protobuf:"1"`
	// Message is the error of the last failed check, while unhealthy.
	Message string `yaml:"message,omitempty" protobuf:"2"`
	// LastChange is when Healthy last changed.
	LastChange time.Time `yaml:"lastChange" protobuf:"3"`
}

// NewContainerHealthStatus initializes a ContainerHealthStatus resource.
func NewContainerHealthStatus(namespace resource.Namespace, id resource.ID) *ContainerHealthStatus {
	return typed.NewResource[ContainerHealthStatusSpec, ContainerHealthStatusExtension](
		resource.NewMetadata(namespace, ContainerHealthStatusType, id, resource.VersionUndefined),
		ContainerHealthStatusSpec{},
	)
}

// ContainerHealthStatusExtension is auxiliary resource data for ContainerHealthStatus.
type ContainerHealthStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerHealthStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerHealthStatusType,
		Aliases:          []resource.Type{"containerhealthstatus", "containerhealthstatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Healthy",
				JSONPath: `{.healthy}`,
			},
			{
				Name:     "Message",
				JSONPath: `{.message}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(ContainerHealthStatusType, &ContainerHealthStatus{}); err != nil {
		panic(err)
	}
}

// CurrentHealthStatus returns the health status of containerID's instance reported by
// containerInstanceStatus, or nil if there is none.
func CurrentHealthStatus(
	ctx context.Context,
	reader controller.Reader,
	containerID string,
	containerInstanceStatus *ContainerInstanceStatus,
) (*ContainerHealthStatus, error) {
	if containerInstanceStatus == nil {
		return nil, nil
	}

	instanceSpecID := InstanceID(containerID, containerInstanceStatus.TypedSpec().Generation)

	containerHealthStatus, err := safe.ReaderGetByID[*ContainerHealthStatus](ctx, reader, instanceSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get health status %q: %w", instanceSpecID, err)
	}

	return containerHealthStatus, nil
}
