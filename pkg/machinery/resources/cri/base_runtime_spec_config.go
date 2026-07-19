// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// BaseRuntimeSpecConfigType is type of BaseRuntimeSpecConfig resource.
const BaseRuntimeSpecConfigType = resource.Type("BaseRuntimeSpecConfigs.cri.talos.dev")

const (
	// BaseRuntimeSpecDefaultID is the generated default OCI runtime spec resource ID.
	BaseRuntimeSpecDefaultID resource.ID = "default"
	// BaseRuntimeSpecOverridesID is the CRIBaseRuntimeSpecConfig document overrides resource ID.
	BaseRuntimeSpecOverridesID resource.ID = "overrides"
)

// BaseRuntimeSpecConfig holds one OCI runtime spec configuration source.
type BaseRuntimeSpecConfig = typed.Resource[BaseRuntimeSpecConfigSpec, BaseRuntimeSpecConfigExtension]

// BaseRuntimeSpecConfigSpec describes an OCI runtime spec configuration source.
//
//gotagsrewrite:gen
type BaseRuntimeSpecConfigSpec struct {
	Object map[string]any `protobuf:"1" yaml:",inline"`
}

// NewBaseRuntimeSpecConfig initializes a BaseRuntimeSpecConfig resource.
func NewBaseRuntimeSpecConfig(id resource.ID) *BaseRuntimeSpecConfig {
	return typed.NewResource[BaseRuntimeSpecConfigSpec, BaseRuntimeSpecConfigExtension](
		resource.NewMetadata(NamespaceName, BaseRuntimeSpecConfigType, id, resource.VersionUndefined),
		BaseRuntimeSpecConfigSpec{},
	)
}

// BaseRuntimeSpecConfigExtension provides auxiliary methods for BaseRuntimeSpecConfig.
type BaseRuntimeSpecConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BaseRuntimeSpecConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BaseRuntimeSpecConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(BaseRuntimeSpecConfigType, &BaseRuntimeSpecConfig{})
	if err != nil {
		panic(err)
	}
}
