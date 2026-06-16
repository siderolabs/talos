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

// BootIDType is type of [BootID] resource.
const BootIDType = resource.Type("BootIDs.runtime.talos.dev")

// BootID resource holds the kernel boot ID (contents of /proc/sys/kernel/random/boot_id).
type BootID = typed.Resource[BootIDSpec, BootIDExtension]

// BootIDID is the singleton resource ID for [BootID].
const BootIDID resource.ID = "boot-id"

// BootIDSpec presents the kernel boot ID (contents of /proc/sys/kernel/random/boot_id).
//
//gotagsrewrite:gen
type BootIDSpec struct {
	BootID string `yaml:"bootID" protobuf:"1"`
}

// NewBootID initializes a [BootID] resource.
func NewBootID() *BootID {
	return typed.NewResource[BootIDSpec, BootIDExtension](
		resource.NewMetadata(NamespaceName, BootIDType, BootIDID, resource.VersionUndefined),
		BootIDSpec{},
	)
}

// BootIDExtension is auxiliary resource data for [BootID].
type BootIDExtension struct{}

// ResourceDefinition implements [meta.ResourceDefinitionProvider] interface.
func (BootIDExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BootIDType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Boot ID",
				JSONPath: "{.bootID}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[BootIDSpec](BootIDType, &BootID{})
	if err != nil {
		panic(err)
	}
}
