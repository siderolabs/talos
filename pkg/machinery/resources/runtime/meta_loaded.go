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

// MetaLoadedType is type of [MetaLoaded] resource.
const MetaLoadedType = resource.Type("MetaLoads.runtime.talos.dev")

// MetaLoaded resource appears when all meta keys are loaded.
type MetaLoaded = typed.Resource[MetaLoadedSpec, MetaLoadedExtension]

// MetaLoadedID is the ID of [MetaLoaded] resource.
const MetaLoadedID = resource.ID("meta-loaded")

// MetaLoadedSpec is the spec for meta loaded. The Done field is always true when resource exists.
//
//gotagsrewrite:gen
type MetaLoadedSpec struct {
	Done bool `yaml:"done" protobuf:"1"`
}

// NewMetaLoaded initializes a [MetaLoaded] resource.
func NewMetaLoaded() *MetaLoaded {
	return typed.NewResource[MetaLoadedSpec, MetaLoadedExtension](
		resource.NewMetadata(NamespaceName, MetaLoadedType, MetaLoadedID, resource.VersionUndefined),
		MetaLoadedSpec{},
	)
}

// MetaLoadedExtension is auxiliary resource data for [MetaLoaded].
type MetaLoadedExtension struct{}

// ResourceDefinition implements [meta.ResourceDefinitionProvider] interface.
func (MetaLoadedExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MetaLoadedType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Done",
				JSONPath: `{.done}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MetaLoadedSpec](MetaLoadedType, &MetaLoaded{})
	if err != nil {
		panic(err)
	}
}
