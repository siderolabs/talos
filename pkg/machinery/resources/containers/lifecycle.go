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

// ContainerLifecycleType is type of ContainerLifecycle resource.
const ContainerLifecycleType = resource.Type("ContainerLifecycles.containers.talos.dev")

// ContainerLifecycleID is the singleton ID of the resource.
const ContainerLifecycleID = resource.ID("containers")

// ContainerLifecycle resource exists so that containers can be stopped gracefully on the way down.
type ContainerLifecycle = typed.Resource[ContainerLifecycleSpec, ContainerLifecycleExtension]

// ContainerLifecycleSpec is the spec for ContainerLifecycle.
//
//gotagsrewrite:gen
type ContainerLifecycleSpec struct{}

// NewContainerLifecycle initializes a ContainerLifecycle resource.
func NewContainerLifecycle(namespace resource.Namespace, id resource.ID) *ContainerLifecycle {
	return typed.NewResource[ContainerLifecycleSpec, ContainerLifecycleExtension](
		resource.NewMetadata(namespace, ContainerLifecycleType, id, resource.VersionUndefined),
		ContainerLifecycleSpec{},
	)
}

// ContainerLifecycleExtension is auxiliary resource data for ContainerLifecycle.
type ContainerLifecycleExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerLifecycleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerLifecycleType,
		Aliases:          []resource.Type{"containerlifecycle", "containerlifecycles"},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(ContainerLifecycleType, &ContainerLifecycle{}); err != nil {
		panic(err)
	}
}
