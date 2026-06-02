// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ImageFactorySchematicType is type of ImageFactorySchematic resource.
const ImageFactorySchematicType = resource.Type("ImageFactorySchematics.runtime.talos.dev")

// ImageFactorySchematicID is the singleton ID for ImageFactorySchematic resource.
const ImageFactorySchematicID resource.ID = "image-factory-schematic"

// ImageFactorySchematic resource holds schematic information injected by Image Factory.
type ImageFactorySchematic = typed.Resource[ImageFactorySchematicSpec, ImageFactorySchematicExtension]

// ImageFactorySchematicSpec describes Image Factory schematic information.
//
//gotagsrewrite:gen
type ImageFactorySchematicSpec struct {
	SchematicID string `yaml:"schematicId" protobuf:"1"`
	Flavor      string `yaml:"flavor" protobuf:"2"`
	APIURL      string `yaml:"apiUrl" protobuf:"3"`
}

// NewImageFactorySchematic initializes an ImageFactorySchematic resource.
func NewImageFactorySchematic(namespace resource.Namespace, id resource.ID) *ImageFactorySchematic {
	return typed.NewResource[ImageFactorySchematicSpec, ImageFactorySchematicExtension](
		resource.NewMetadata(namespace, ImageFactorySchematicType, id, resource.VersionUndefined),
		ImageFactorySchematicSpec{},
	)
}

// ImageFactorySchematicExtension provides auxiliary methods for ImageFactorySchematic.
type ImageFactorySchematicExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ImageFactorySchematicExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ImageFactorySchematicType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Schematic ID",
				JSONPath: `{.schematicId}`,
			},
			{
				Name:     "Flavor",
				JSONPath: `{.flavor}`,
			},
			{
				Name:     "API URL",
				JSONPath: `{.apiUrl}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ImageFactorySchematicSpec](ImageFactorySchematicType, &ImageFactorySchematic{})
	if err != nil {
		panic(err)
	}
}
