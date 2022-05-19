// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// ProcessorType is type of Processor resource.
const ProcessorType = resource.Type("Processors.hardware.talos.dev")

// Processor resource holds node Processor information.
type Processor = typed.Resource[ProcessorSpec, ProcessorRD]

// ProcessorSpec represents a single processor.
type ProcessorSpec struct {
	Socket       string `yaml:"socket,omitempty"`
	Manufacturer string `yaml:"manufacturer,omitempty"`
	ProductName  string `yaml:"productName,omitempty"`
	// MaxSpeed is in megahertz (Mhz)
	MaxSpeed uint32 `yaml:"maxSpeedMhz,omitempty"`
	// Speed is in megahertz (Mhz)
	BootSpeed    uint32 `yaml:"bootSpeedMhz,omitempty"`
	Status       uint32 `yaml:"status,omitempty"`
	SerialNumber string `yaml:"serialNumber,omitempty"`
	AssetTag     string `yaml:"assetTag,omitempty"`
	PartNumber   string `yaml:"partNumber,omitempty"`
	CoreCount    uint32 `yaml:"coreCount,omitempty"`
	CoreEnabled  uint32 `yaml:"coreEnabled,omitempty"`
	ThreadCount  uint32 `yaml:"threadCount,omitempty"`
}

// NewProcessorInfo initializes a ProcessorInfo resource.
func NewProcessorInfo(id string) *Processor {
	return typed.NewResource[ProcessorSpec, ProcessorRD](
		resource.NewMetadata(NamespaceName, ProcessorType, id, resource.VersionUndefined),
		ProcessorSpec{},
	)
}

// ProcessorRD provides auxiliary methods for Processor info.
type ProcessorRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (c ProcessorRD) ResourceDefinition(resource.Metadata, ProcessorSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: ProcessorType,
		Aliases: []resource.Type{
			"cpus",
			"cpu",
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
				Name:     "Cores",
				JSONPath: `{.coreCount}`,
			},
			{
				Name:     "Threads",
				JSONPath: `{.threadCount}`,
			},
		},
	}
}
