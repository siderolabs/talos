// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// SystemDiskType is type of SystemDisk resource.
const SystemDiskType = resource.Type("SystemDisks.block.talos.dev")

// SystemDisk resource holds ID of the disk which is the Talos system disk.
type SystemDisk = typed.Resource[SystemDiskSpec, SystemDiskExtension]

// SystemDiskID is the singleton resource ID.
const SystemDiskID resource.ID = "system-disk"

// SystemDiskSpec is the spec for SystemDisks resource.
//
//gotagsrewrite:gen
type SystemDiskSpec struct {
	DiskID  string `yaml:"diskID" protobuf:"1"`
	DevPath string `yaml:"devPath" protobuf:"2"`
}

// NewSystemDisk initializes a BlockSystemDisk resource.
func NewSystemDisk(namespace resource.Namespace, id resource.ID) *SystemDisk {
	return typed.NewResource[SystemDiskSpec, SystemDiskExtension](
		resource.NewMetadata(namespace, SystemDiskType, id, resource.VersionUndefined),
		SystemDiskSpec{},
	)
}

// SystemDiskExtension is auxiliary resource data for BlockSystemDisk.
type SystemDiskExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SystemDiskExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SystemDiskType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Disk",
				JSONPath: `{.diskID}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SystemDiskSpec](SystemDiskType, &SystemDisk{})
	if err != nil {
		panic(err)
	}
}
