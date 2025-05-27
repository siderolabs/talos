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

// SwapStatusType is type of SwapStatus resource.
const SwapStatusType = resource.Type("SwapStatuses.block.talos.dev")

// SwapStatus resource holds a list of stable symlinks to the blockdevice.
type SwapStatus = typed.Resource[SwapStatusSpec, SwapStatusExtension]

// SwapStatusSpec is the spec for SwapStatuss resource.
//
//gotagsrewrite:gen
type SwapStatusSpec struct {
	Device    string `yaml:"device" protobuf:"1"`
	Type      string `yaml:"type" protobuf:"7"`
	SizeBytes uint64 `yaml:"size" protobuf:"2"`
	SizeHuman string `yaml:"sizeHuman" protobuf:"3"`
	UsedBytes uint64 `yaml:"used" protobuf:"4"`
	UsedHuman string `yaml:"usedHuman" protobuf:"5"`
	Priority  int32  `yaml:"priority" protobuf:"6"`
}

// NewSwapStatus initializes a SwapStatus resource.
func NewSwapStatus(namespace resource.Namespace, id resource.ID) *SwapStatus {
	return typed.NewResource[SwapStatusSpec, SwapStatusExtension](
		resource.NewMetadata(namespace, SwapStatusType, id, resource.VersionUndefined),
		SwapStatusSpec{},
	)
}

// SwapStatusExtension is auxiliary resource data for SwapStatus.
type SwapStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SwapStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SwapStatusType,
		Aliases:          []resource.Type{"swap", "swaps"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Device",
				JSONPath: "{.device}",
			},
			{
				Name:     "Size",
				JSONPath: "{.sizeHuman}",
			},
			{
				Name:     "Used",
				JSONPath: "{.usedHuman}",
			},
			{
				Name:     "Priority",
				JSONPath: "{.priority}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SwapStatusSpec](SwapStatusType, &SwapStatus{})
	if err != nil {
		panic(err)
	}
}
