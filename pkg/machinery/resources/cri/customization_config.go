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

// CustomizationConfigType is type of CustomizationConfig resource.
const CustomizationConfigType = resource.Type("CustomizationConfigs.cri.talos.dev")

// CustomizationConfig holds one CRI customization fragment.
type CustomizationConfig = typed.Resource[CustomizationConfigSpec, CustomizationConfigExtension]

// CustomizationConfigSpec describes one CRI customization fragment.
//
//gotagsrewrite:gen
type CustomizationConfigSpec struct {
	Content string `yaml:"content" protobuf:"1"`
}

// NewCustomizationConfig initializes a CustomizationConfig resource.
func NewCustomizationConfig(id resource.ID) *CustomizationConfig {
	return typed.NewResource[CustomizationConfigSpec, CustomizationConfigExtension](
		resource.NewMetadata(NamespaceName, CustomizationConfigType, id, resource.VersionUndefined),
		CustomizationConfigSpec{},
	)
}

// CustomizationConfigExtension provides auxiliary methods for CustomizationConfig.
type CustomizationConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (CustomizationConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CustomizationConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(CustomizationConfigType, &CustomizationConfig{})
	if err != nil {
		panic(err)
	}
}
