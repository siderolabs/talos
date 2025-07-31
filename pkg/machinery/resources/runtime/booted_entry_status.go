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

// BootedEntryType is the type of booted entry resource.
const BootedEntryType = resource.Type("BootedEntries.talos.dev")

// BootedEntryID is the ID of the booted entry resource.
const BootedEntryID = resource.ID("bootedentry")

// BootedEntry is the booted entry resource.
type BootedEntry = typed.Resource[BootedEntrySpec, BootedEntryExtension]

// BootedEntrySpec describes the booted entry resource properties.
//
//gotagsrewrite:gen
type BootedEntrySpec struct {
	BootedEntry string `yaml:"bootedEntry,omitempty" protobuf:"1"`
}

// BootedEntryExtension provides auxiliary methods for BootedEntry resource.
type BootedEntryExtension struct{}

// NewBootedEntrySpec initializes a new BootedEntrySpec.
func NewBootedEntrySpec() *BootedEntry {
	return typed.NewResource[BootedEntrySpec, BootedEntryExtension](
		resource.NewMetadata(NamespaceName, BootedEntryType, BootedEntryID, resource.VersionUndefined),
		BootedEntrySpec{},
	)
}

// ResourceDefinition implements [typed.Extension] interface.
func (BootedEntryExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BootedEntryType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Booted Entry",
				JSONPath: `{.bootedEntry}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(BootedEntryType, &BootedEntry{})
	if err != nil {
		panic(err)
	}
}
