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

// ContainerImageStatusType is type of ContainerImageStatus resource.
const ContainerImageStatusType = resource.Type("ContainerImageStatuses.containers.talos.dev")

// ContainerImageStatus resource holds the state of a container's image pull.
type ContainerImageStatus = typed.Resource[ContainerImageStatusSpec, ContainerImageStatusExtension]

// ContainerImageStatusSpec is the spec for ContainerImageStatus.
//
//gotagsrewrite:gen
type ContainerImageStatusSpec struct {
	Phase ContainerImagePhase `yaml:"phase" protobuf:"1"`
	// Image is the reference that was requested, in canonical form.
	Image string `yaml:"image" protobuf:"2"`
	// Digest is the resolved digest, set once the pull completes.
	Digest string `yaml:"digest,omitempty" protobuf:"3"`
	// Error is the last pull failure, verbatim.
	Error string `yaml:"error,omitempty" protobuf:"4"`
}

// NewContainerImageStatus initializes a ContainerImageStatus resource.
func NewContainerImageStatus(namespace resource.Namespace, id resource.ID) *ContainerImageStatus {
	return typed.NewResource[ContainerImageStatusSpec, ContainerImageStatusExtension](
		resource.NewMetadata(namespace, ContainerImageStatusType, id, resource.VersionUndefined),
		ContainerImageStatusSpec{},
	)
}

// ContainerImageStatusExtension is auxiliary resource data for ContainerImageStatus.
type ContainerImageStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerImageStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerImageStatusType,
		Aliases:          []resource.Type{"containerimagestatus", "containerimagestatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Phase",
				JSONPath: `{.phase}`,
			},
			{
				Name:     "Digest",
				JSONPath: `{.digest}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ContainerImageStatusType, &ContainerImageStatus{})
	if err != nil {
		panic(err)
	}
}
