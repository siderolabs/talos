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

// KernelModuleStatusType is type of KernelModuleStatus resource.
const KernelModuleStatusType = resource.Type("KernelModuleStatuses.runtime.talos.dev")

// KernelModuleStatus resource holds information about all Linux kernel modules (dynamically loaded and built-in).
type KernelModuleStatus = typed.Resource[KernelModuleStatusSpec, KernelModuleStatusExtension]

// KernelModuleStatusSpec represents the status of a Linux kernel module.
//
//gotagsrewrite:gen
type KernelModuleStatusSpec struct {
	// Type indicates whether the kernel module is built-in or dynamically loaded.
	Type KernelModuleType `yaml:"type" protobuf:"1"`
	// Size is the size of the kernel module in bytes.
	Size int `yaml:"size" protobuf:"2"`
	// ReferenceCount is the number of references to this kernel module.
	ReferenceCount int `yaml:"referenceCount" protobuf:"3"`
	// Dependencies lists the names of kernel modules that this module depends on.
	Dependencies []string `yaml:"dependencies,omitempty" protobuf:"4"`
	// State is the operational state of the kernel module.
	State KernelModuleState `yaml:"state" protobuf:"5"`
	// Address is the memory address where the kernel module is loaded (if applicable).
	Address string `yaml:"address" protobuf:"6"`
}

// NewKernelModuleStatus initializes a KernelModuleStatus resource.
func NewKernelModuleStatus(namespace resource.Namespace, id resource.ID) *KernelModuleStatus {
	return typed.NewResource[KernelModuleStatusSpec, KernelModuleStatusExtension](
		resource.NewMetadata(namespace, KernelModuleStatusType, id, resource.VersionUndefined),
		KernelModuleStatusSpec{},
	)
}

// KernelModuleStatusExtension is auxiliary resource data for KernelModuleStatus.
type KernelModuleStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KernelModuleStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelModuleStatusType,
		Aliases:          []resource.Type{"module", "modules"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Type",
				JSONPath: "{.type}",
			},
			{
				Name:     "State",
				JSONPath: "{.state}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KernelModuleStatusSpec](KernelModuleStatusType, &KernelModuleStatus{})
	if err != nil {
		panic(err)
	}
}
