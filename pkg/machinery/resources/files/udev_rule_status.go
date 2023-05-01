// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// UdevRuleStatusType is type of UdevRules resource.
const UdevRuleStatusType = resource.Type("UdevRuleStatuses.files.talos.dev")

// UdevRuleStatus is a resource for UdevRule.
type UdevRuleStatus = typed.Resource[UdevRuleStatusSpec, UdevRuleStatusRD]

// UdevRuleStatusSpec is the specification for UdevRule resource.
//
//gotagsrewrite:gen
type UdevRuleStatusSpec struct {
	Active bool `yaml:"active" protobuf:"1"`
}

// NewUdevRuleStatus initializes a new UdevRule resource.
func NewUdevRuleStatus(id string) *UdevRuleStatus {
	return typed.NewResource[UdevRuleStatusSpec, UdevRuleStatusRD](
		resource.NewMetadata(NamespaceName, UdevRuleStatusType, id, resource.VersionUndefined),
		UdevRuleStatusSpec{},
	)
}

// UdevRuleStatusRD provides auxiliary methods for UdevRules.
type UdevRuleStatusRD struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (UdevRuleStatusRD) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             UdevRuleStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[UdevRuleStatusSpec](UdevRuleStatusType, &UdevRuleStatus{})
	if err != nil {
		panic(err)
	}
}
