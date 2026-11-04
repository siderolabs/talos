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

// SELinuxBooleanStatusType is type of SELinuxBooleanStatus resource.
const SELinuxBooleanStatusType = resource.Type("SELinuxBooleanStatuses.talos.dev")

// SELinuxBooleanStatus reports an SELinux boolean of the loaded policy, or a boolean of the machine config the policy
// does not define.
type SELinuxBooleanStatus = typed.Resource[SELinuxBooleanStatusSpec, SELinuxBooleanStatusExtension]

// SELinuxBooleanStatusSpec describes the SELinuxBooleanStatus resource.
//
//gotagsrewrite:gen
type SELinuxBooleanStatusSpec struct {
	// Value is the value of the boolean in the kernel.
	Value bool `yaml:"value" protobuf:"1"`
	// Default is the value of the boolean when the policy was loaded.
	Default bool `yaml:"default" protobuf:"2"`
	// Configured is true when the machine config sets the boolean.
	Configured bool `yaml:"configured" protobuf:"3"`
	// Unknown is true when the machine config sets a boolean the loaded policy does not define.
	Unknown bool `yaml:"unknown,omitempty" protobuf:"4"`
}

// NewSELinuxBooleanStatus initializes a SELinuxBooleanStatus resource.
func NewSELinuxBooleanStatus(name string) *SELinuxBooleanStatus {
	return typed.NewResource[SELinuxBooleanStatusSpec, SELinuxBooleanStatusExtension](
		resource.NewMetadata(NamespaceName, SELinuxBooleanStatusType, name, resource.VersionUndefined),
		SELinuxBooleanStatusSpec{},
	)
}

// SELinuxBooleanStatusExtension provides auxiliary methods for SELinuxBooleanStatus.
type SELinuxBooleanStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SELinuxBooleanStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SELinuxBooleanStatusType,
		Aliases:          []resource.Type{"selinuxbooleans", "selinuxboolean"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Value",
				JSONPath: `{.value}`,
			},
			{
				Name:     "Default",
				JSONPath: `{.default}`,
			},
			{
				Name:     "Configured",
				JSONPath: `{.configured}`,
			},
			{
				Name:     "Unknown",
				JSONPath: `{.unknown}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SELinuxBooleanStatusSpec](SELinuxBooleanStatusType, &SELinuxBooleanStatus{})
	if err != nil {
		panic(err)
	}
}
