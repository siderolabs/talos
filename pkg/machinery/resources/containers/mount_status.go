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

// ContainerMountStatusType is type of ContainerMountStatus resource.
const ContainerMountStatusType = resource.Type("ContainerMountStatuses.containers.talos.dev")

// ContainerMountStatus resource holds a container's mounts with their host paths resolved.
//
// The ID is the container name.
type ContainerMountStatus = typed.Resource[ContainerMountStatusSpec, ContainerMountStatusExtension]

// ContainerMountStatusSpec is the spec for ContainerMountStatus.
//
//gotagsrewrite:gen
type ContainerMountStatusSpec struct {
	// Ready is true once every mount the container declares is available.
	Ready bool `yaml:"ready" protobuf:"1"`
	// Mounts are the resolved mounts, with host source paths filled in. Only meaningful when Ready.
	Mounts []ResolvedMountSpec `yaml:"mounts,omitempty" protobuf:"2"`
	// Error describes why the mounts are not ready.
	Error string `yaml:"error,omitempty" protobuf:"3"`
}

// NewContainerMountStatus initializes a ContainerMountStatus resource.
func NewContainerMountStatus(namespace resource.Namespace, id resource.ID) *ContainerMountStatus {
	return typed.NewResource[ContainerMountStatusSpec, ContainerMountStatusExtension](
		resource.NewMetadata(namespace, ContainerMountStatusType, id, resource.VersionUndefined),
		ContainerMountStatusSpec{},
	)
}

// ContainerMountStatusExtension is auxiliary resource data for ContainerMountStatus.
type ContainerMountStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerMountStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerMountStatusType,
		Aliases:          []resource.Type{"containermountstatus", "containermountstatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: `{.ready}`,
			},
			{
				Name:     "Error",
				JSONPath: `{.error}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ContainerMountStatusType, &ContainerMountStatus{})
	if err != nil {
		panic(err)
	}
}
