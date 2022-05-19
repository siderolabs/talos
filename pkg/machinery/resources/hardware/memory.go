// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// MemoryType is type of Memory resource.
const MemoryType = resource.Type("Memories.hardware.talos.dev")

// MemoryInfo resource holds node MemoryInfo information.
type MemoryInfo = typed.Resource[MemorySpec, MemoryRD]

// MemorySpec represents a single Memory.
type MemorySpec struct {
	Size          uint32 `yaml:"size"`
	DeviceLocator string `yaml:"deviceLocator"`
	BankLocator   string `yaml:"bankLocator"`
	Speed         uint32 `yaml:"speed"`
	Manufacturer  string `yaml:"manufacturer,omitempty"`
	SerialNumber  string `yaml:"serialNumber,omitempty"`
	AssetTag      string `yaml:"assetTag,omitempty"`
	ProductName   string `yaml:"productName,omitempty"`
}

// NewMemoryInfo initializes a MemoryInfo resource.
func NewMemoryInfo(id string) *MemoryInfo {
	return typed.NewResource[MemorySpec, MemoryRD](
		resource.NewMetadata(NamespaceName, MemoryType, id, resource.VersionUndefined),
		MemorySpec{},
	)
}

// MemoryRD provides auxiliary methods for Memory info.
type MemoryRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (c MemoryRD) ResourceDefinition(resource.Metadata, MemorySpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: MemoryType,
		Aliases: []resource.Type{
			"memory",
			"ram",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Manufacturer",
				JSONPath: `{.manufacturer}`,
			},
			{
				Name:     "Model",
				JSONPath: `{.productName}`,
			},
			{
				Name:     "Size",
				JSONPath: `{.size}`,
			},
		},
	}
}
