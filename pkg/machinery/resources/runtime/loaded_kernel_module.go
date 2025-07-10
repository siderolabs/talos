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

// LoadedKernelModuleType is type of LoadedKernelModule resource.
const LoadedKernelModuleType = resource.Type("LoadedKernelModules.runtime.talos.dev")

// LoadedKernelModule resource holds information about Linux kernel module to load.
type LoadedKernelModule = typed.Resource[LoadedKernelModuleSpec, LoadedKernelModuleExtension]

// LoadedKernelModuleSpec describes Linux kernel module to load.
//
//gotagsrewrite:gen
type LoadedKernelModuleSpec struct {
	Size           int      `yaml:"size" protobuf:"1"`
	ReferenceCount int      `yaml:"referenceCount" protobuf:"2"`
	Dependencies   []string `yaml:"dependencies,omitempty" protobuf:"3"`
	State          string   `yaml:"state" protobuf:"4"`
	Address        string   `yaml:"address" protobuf:"5"`
}

// NewLoadedKernelModule initializes a LoadedKernelModule resource.
func NewLoadedKernelModule(namespace resource.Namespace, id resource.ID) *LoadedKernelModule {
	return typed.NewResource[LoadedKernelModuleSpec, LoadedKernelModuleExtension](
		resource.NewMetadata(namespace, LoadedKernelModuleType, id, resource.VersionUndefined),
		LoadedKernelModuleSpec{},
	)
}

// LoadedKernelModuleExtension is auxiliary resource data for LoadedKernelModule.
type LoadedKernelModuleExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LoadedKernelModuleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LoadedKernelModuleType,
		Aliases:          []resource.Type{"module", "modules"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "State",
				JSONPath: "{.state}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LoadedKernelModuleSpec](LoadedKernelModuleType, &LoadedKernelModule{})
	if err != nil {
		panic(err)
	}
}
