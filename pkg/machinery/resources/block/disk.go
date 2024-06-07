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

// DiskType is type of Disk resource.
const DiskType = resource.Type("Disks.block.talos.dev")

// Disk resource holds status of hardware disks.
type Disk = typed.Resource[DiskSpec, DiskExtension]

// DiskSpec is the spec for Disks status.
//
//gotagsrewrite:gen
type DiskSpec struct {
	Size       uint64 `yaml:"size" protobuf:"1"`
	IOSize     uint   `yaml:"ioSize" protobuf:"2"`
	SectorSize uint   `yaml:"sectorSize" protobuf:"3"`

	Readonly bool `yaml:"readonly" protobuf:"4"`

	Model      string `yaml:"model,omitempty" protobuf:"5"`
	Serial     string `yaml:"serial,omitempty" protobuf:"6"`
	Modalias   string `yaml:"modalias,omitempty" protobuf:"7"`
	WWID       string `yaml:"wwid,omitempty" protobuf:"8"`
	BusPath    string `yaml:"busPath,omitempty" protobuf:"9"`
	SubSystem  string `yaml:"subSystem,omitempty" protobuf:"10"`
	Transport  string `yaml:"transport,omitempty" protobuf:"11"`
	Rotational bool   `yaml:"rotational,omitempty" protobuf:"12"`
}

// NewDisk initializes a BlockDisk resource.
func NewDisk(namespace resource.Namespace, id resource.ID) *Disk {
	return typed.NewResource[DiskSpec, DiskExtension](
		resource.NewMetadata(namespace, DiskType, id, resource.VersionUndefined),
		DiskSpec{},
	)
}

// DiskExtension is auxiliary resource data for BlockDisk.
type DiskExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DiskExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiskType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Size",
				JSONPath: `{.size}`,
			},
			{
				Name:     "Read Only",
				JSONPath: `{.readonly}`,
			},
			{
				Name:     "Transport",
				JSONPath: `{.transport}`,
			},
			{
				Name:     "Rotational",
				JSONPath: `{.rotational}`,
			},
			{
				Name:     "WWID",
				JSONPath: `{.wwid}`,
			},
			{
				Name:     "Model",
				JSONPath: `{.model}`,
			},
			{
				Name:     "Serial",
				JSONPath: `{.serial}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[DiskSpec](DiskType, &Disk{})
	if err != nil {
		panic(err)
	}
}
