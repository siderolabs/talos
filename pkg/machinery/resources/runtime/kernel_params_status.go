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

// KernelParamStatusType is type of KernelParam resource.
const KernelParamStatusType = resource.Type("KernelParamStatuses.runtime.talos.dev")

// KernelParamStatus resource holds defined sysctl flags status.
type KernelParamStatus = typed.Resource[KernelParamStatusSpec, KernelParamStatusRD]

// KernelParamStatusSpec describes status of the defined sysctls.
//
//gotagsrewrite:gen
type KernelParamStatusSpec struct {
	Current     string `yaml:"current" protobuf:"1"`
	Default     string `yaml:"default" protobuf:"2"`
	Unsupported bool   `yaml:"unsupported" protobuf:"3"`
}

// NewKernelParamStatus initializes a KernelParamStatus resource.
func NewKernelParamStatus(namespace resource.Namespace, id resource.ID) *KernelParamStatus {
	return typed.NewResource[KernelParamStatusSpec, KernelParamStatusRD](
		resource.NewMetadata(namespace, KernelParamStatusType, id, resource.VersionUndefined),
		KernelParamStatusSpec{},
	)
}

// KernelParamStatusRD is auxiliary resource data for KernelParamStatus.
type KernelParamStatusRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KernelParamStatusRD) ResourceDefinition(resource.Metadata, KernelParamStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelParamStatusType,
		Aliases:          []resource.Type{"sysctls", "kernelparameters", "kernelparams"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Current",
				JSONPath: `{.current}`,
			},
			{
				Name:     "Default",
				JSONPath: `{.default}`,
			},
			{
				Name:     "Unsupported",
				JSONPath: `{.unsupported}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KernelParamStatusSpec](KernelParamStatusType, &KernelParamStatus{})
	if err != nil {
		panic(err)
	}
}
