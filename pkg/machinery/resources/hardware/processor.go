// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ProcessorType is type of Processor resource.
const ProcessorType = resource.Type("Processors.hardware.talos.dev")

// Processor resource holds node Processor information.
type Processor = typed.Resource[ProcessorSpec, ProcessorExtension]

// ProcessorSpec represents a single processor.
//
//gotagsrewrite:gen
type ProcessorSpec struct {
	Socket       string `yaml:"socket,omitempty" protobuf:"1"`
	Manufacturer string `yaml:"manufacturer,omitempty" protobuf:"2"`
	ProductName  string `yaml:"productName,omitempty" protobuf:"3"`
	// MaxSpeed is in megahertz (Mhz)
	MaxSpeed uint32 `yaml:"maxSpeedMhz,omitempty" protobuf:"4"`
	// Speed is in megahertz (Mhz)
	BootSpeed    uint32 `yaml:"bootSpeedMhz,omitempty" protobuf:"5"`
	Status       uint32 `yaml:"status,omitempty" protobuf:"6"`
	SerialNumber string `yaml:"serialNumber,omitempty" protobuf:"7"`
	AssetTag     string `yaml:"assetTag,omitempty" protobuf:"8"`
	PartNumber   string `yaml:"partNumber,omitempty" protobuf:"9"`
	CoreCount    uint32 `yaml:"coreCount,omitempty" protobuf:"10"`
	CoreEnabled  uint32 `yaml:"coreEnabled,omitempty" protobuf:"11"`
	ThreadCount  uint32 `yaml:"threadCount,omitempty" protobuf:"12"`
}

// NewProcessorInfo initializes a ProcessorInfo resource.
func NewProcessorInfo(id string) *Processor {
	return typed.NewResource[ProcessorSpec, ProcessorExtension](
		resource.NewMetadata(NamespaceName, ProcessorType, id, resource.VersionUndefined),
		ProcessorSpec{},
	)
}

// ProcessorExtension provides auxiliary methods for Processor info.
type ProcessorExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ProcessorExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
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

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ProcessorSpec](ProcessorType, &Processor{})
	if err != nil {
		panic(err)
	}
}
