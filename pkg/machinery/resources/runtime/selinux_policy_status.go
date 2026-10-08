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

// SELinuxPolicyStatusType is type of SELinuxPolicyStatus resource.
const SELinuxPolicyStatusType = resource.Type("SELinuxPolicyStatuses.talos.dev")

// SELinuxPolicyStatusID is the ID of the single SELinuxPolicyStatus resource.
const SELinuxPolicyStatusID = resource.ID("selinux")

// SELinuxPolicyStatus reports the SELinux policy modules compiled into the loaded policy.
type SELinuxPolicyStatus = typed.Resource[SELinuxPolicyStatusSpec, SELinuxPolicyStatusExtension]

// SELinuxPolicyStatusSpec describes the SELinuxPolicyStatus resource.
//
//gotagsrewrite:gen
type SELinuxPolicyStatusSpec struct {
	// Modules are the SELinuxPolicyConfig documents of the machine config at the last reconcile.
	Modules []string `yaml:"modules" protobuf:"1"`
	// Error names the modules the compiler rejected at the last reconcile, with its error; the other modules are loaded.
	Error string `yaml:"error,omitempty" protobuf:"2"`
}

// NewSELinuxPolicyStatus initializes a SELinuxPolicyStatus resource.
func NewSELinuxPolicyStatus() *SELinuxPolicyStatus {
	return typed.NewResource[SELinuxPolicyStatusSpec, SELinuxPolicyStatusExtension](
		resource.NewMetadata(NamespaceName, SELinuxPolicyStatusType, SELinuxPolicyStatusID, resource.VersionUndefined),
		SELinuxPolicyStatusSpec{},
	)
}

// SELinuxPolicyStatusExtension provides auxiliary methods for SELinuxPolicyStatus.
type SELinuxPolicyStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SELinuxPolicyStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SELinuxPolicyStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Modules",
				JSONPath: `{.modules}`,
			},
			{
				Name:     "Error",
				JSONPath: `{.error}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SELinuxPolicyStatusSpec](SELinuxPolicyStatusType, &SELinuxPolicyStatus{})
	if err != nil {
		panic(err)
	}
}
