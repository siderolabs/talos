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

// UdevRuleType is type of UdevRules resource.
const UdevRuleType = resource.Type("UdevRules.files.talos.dev")

// UdevRule is a resource for UdevRule.
type UdevRule = typed.Resource[UdevRuleSpec, UdevRuleRD]

// UdevRuleSpec is the specification for UdevRule resource.
//
//gotagsrewrite:gen
type UdevRuleSpec struct {
	Rule string `yaml:"rule" protobuf:"1"`
}

// NewUdevRule initializes a new UdevRule resource.
func NewUdevRule(id string) *UdevRule {
	return typed.NewResource[UdevRuleSpec, UdevRuleRD](
		resource.NewMetadata(NamespaceName, UdevRuleType, id, resource.VersionUndefined),
		UdevRuleSpec{},
	)
}

// UdevRuleRD provides auxiliary methods for UdevRules.
type UdevRuleRD struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (UdevRuleRD) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             UdevRuleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[UdevRuleSpec](UdevRuleType, &UdevRule{})
	if err != nil {
		panic(err)
	}
}
