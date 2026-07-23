// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MDArraySpecType is the type of MDArraySpec resource.
const MDArraySpecType = resource.Type("MDArraySpecs.storage.talos.dev")

// MDArraySpec is the desired state for an MD (software RAID) array.
type MDArraySpec = typed.Resource[MDArraySpecSpec, MDArraySpecExtension]

// MDArraySpecSpec is the spec for MDArraySpec resource.
//
//gotagsrewrite:gen
type MDArraySpecSpec struct {
	// Level is the RAID level.
	Level MDLevel `yaml:"level" protobuf:"1"`
	// VolumeSelector matches the member volumes of the array.
	VolumeSelector cel.Expression `yaml:"volumeSelector" protobuf:"2"`
	// Metadata is the on-disk MD metadata format.
	Metadata MDMetadata `yaml:"metadata" protobuf:"3"`
}

// NewMDArraySpec initializes an MDArraySpec resource.
func NewMDArraySpec(namespace resource.Namespace, id resource.ID) *MDArraySpec {
	return typed.NewResource[MDArraySpecSpec, MDArraySpecExtension](
		resource.NewMetadata(namespace, MDArraySpecType, id, resource.VersionUndefined),
		MDArraySpecSpec{},
	)
}

// MDArraySpecExtension is auxiliary resource data for MDArraySpec.
type MDArraySpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MDArraySpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MDArraySpecType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Level", JSONPath: "{.level}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(MDArraySpecType, &MDArraySpec{}); err != nil {
		panic(err)
	}
}
