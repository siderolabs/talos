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

// SBOMItemType is the type of the SBOM item resource.
const SBOMItemType = resource.Type("SBOMItems.talos.dev")

// SBOMItem is the SBOM item resource.
type SBOMItem = typed.Resource[SBOMItemSpec, SBOMItemExtension]

// SBOMItemSpec describes the SBOM item resource properties.
//
//gotagsrewrite:gen
type SBOMItemSpec struct {
	Name      string   `yaml:"name" protobuf:"1"`
	Version   string   `yaml:"version" protobuf:"2"`
	License   string   `yaml:"license,omitempty" protobuf:"3"`
	CPEs      []string `yaml:"cpes,omitempty" protobuf:"4"`
	PURLs     []string `yaml:"purls,omitempty" protobuf:"5"`
	Extension bool     `yaml:"extension,omitempty" protobuf:"6"`
}

// NewSBOMItemSpec initializes a security state resource.
func NewSBOMItemSpec(namespace resource.Namespace, id resource.ID) *SBOMItem {
	return typed.NewResource[SBOMItemSpec, SBOMItemExtension](
		resource.NewMetadata(namespace, SBOMItemType, id, resource.VersionUndefined),
		SBOMItemSpec{},
	)
}

// SBOMItemExtension provides auxiliary methods for SBOMItem.
type SBOMItemExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SBOMItemExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SBOMItemType,
		DefaultNamespace: NamespaceName,
		Aliases:          []string{"sbom", "sboms"},
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Version",
				JSONPath: `{.version}`,
			},
			{
				Name:     "License",
				JSONPath: `{.license}`,
			},
			{
				Name:     "Extension",
				JSONPath: `{.extension}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SBOMItemSpec](SBOMItemType, &SBOMItem{})
	if err != nil {
		panic(err)
	}
}
