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

// NUMATopologyType is the type of NUMATopology resource.
const NUMATopologyType = resource.Type("NUMATopologies.hardware.talos.dev")

// NUMATopologyID is the singleton topology ID.
const NUMATopologyID = resource.ID("numa")

// NUMATopology contains the Linux NUMA inventory. CPU IDs join CPUCore.LogicalCPUs.
type NUMATopology = typed.Resource[NUMATopologySpec, NUMATopologyExtension]

// NUMATopologySpec contains sorted node and logical CPU inventories.
//
//gotagsrewrite:gen
type NUMATopologySpec struct {
	Nodes       []NUMANodeSpec `yaml:"nodes" protobuf:"1"`
	PresentCPUs []uint32       `yaml:"presentCPUs" protobuf:"2"`
	OnlineCPUs  []uint32       `yaml:"onlineCPUs" protobuf:"3"`
}

// NUMANodeSpec includes CPUless and memoryless nodes.
//
//gotagsrewrite:gen
type NUMANodeSpec struct {
	CPUs             []uint32 `yaml:"cpus" protobuf:"2"`
	MemoryTotalBytes uint64   `yaml:"memoryTotalBytes" protobuf:"3"`
	ID               uint32   `yaml:"id" protobuf:"1"`
}

// NewNUMATopology initializes the singleton resource.
func NewNUMATopology() *NUMATopology {
	return typed.NewResource[NUMATopologySpec, NUMATopologyExtension](resource.NewMetadata(NamespaceName, NUMATopologyType, NUMATopologyID, resource.VersionUndefined), NUMATopologySpec{})
}

// NUMATopologyExtension provides auxiliary methods for NUMATopology.
type NUMATopologyExtension struct{}

// ResourceDefinition implements typed.Extension.
func (NUMATopologyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{Type: NUMATopologyType, DefaultNamespace: NamespaceName, Aliases: []resource.Type{"numa"}}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[NUMATopologySpec](NUMATopologyType, &NUMATopology{}); err != nil {
		panic(err)
	}
}
