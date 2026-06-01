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

// UserDiskConfigStatusType is type of UserDiskConfigStatus resource.
const UserDiskConfigStatusType = resource.Type("UserDiskConfigStatuses.block.talos.dev")

// UserDiskConfigStatus resource holds a status of user disk machine configuration.
type UserDiskConfigStatus = typed.Resource[UserDiskConfigStatusSpec, UserDiskConfigStatusExtension]

// UserDiskConfigStatusID is the ID of the UserDiskConfigStatus resource.
const UserDiskConfigStatusID = "user-disks"

// UserDiskConfigStatusSpec is the spec for UserDiskConfigStatus resource.
//
//gotagsrewrite:gen
type UserDiskConfigStatusSpec struct {
	Ready    bool `yaml:"ready" protobuf:"1"`
	TornDown bool `yaml:"tornDown" protobuf:"2"`
}

// NewUserDiskConfigStatus initializes a UserDiskConfigStatus resource.
func NewUserDiskConfigStatus(namespace resource.Namespace, id resource.ID) *UserDiskConfigStatus {
	return typed.NewResource[UserDiskConfigStatusSpec, UserDiskConfigStatusExtension](
		resource.NewMetadata(namespace, UserDiskConfigStatusType, id, resource.VersionUndefined),
		UserDiskConfigStatusSpec{},
	)
}

// UserDiskConfigStatusExtension is auxiliary resource data for UserDiskConfigStatus.
type UserDiskConfigStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (UserDiskConfigStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             UserDiskConfigStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: `{.ready}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(UserDiskConfigStatusType, &UserDiskConfigStatus{})
	if err != nil {
		panic(err)
	}
}
