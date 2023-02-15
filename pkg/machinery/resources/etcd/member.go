// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MemberType is type of Member resource.
const MemberType = resource.Type("EtcdMembers.etcd.talos.dev")

// LocalMemberID is resource ID for Member resource for etcd.
const LocalMemberID = resource.ID("local")

// Member resource holds status of rendered secrets.
type Member = typed.Resource[MemberSpec, MemberExtension]

// MemberSpec holds information about an etcd member.
//
//gotagsrewrite:gen
type MemberSpec struct {
	MemberID string `yaml:"memberID" protobuf:"1"`
}

// NewMember initializes a Member resource.
func NewMember(namespace resource.Namespace, id resource.ID) *Member {
	return typed.NewResource[MemberSpec, MemberExtension](
		resource.NewMetadata(namespace, MemberType, id, resource.VersionUndefined),
		MemberSpec{},
	)
}

// MemberExtension provides auxiliary methods for Member.
type MemberExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MemberExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MemberType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Member ID",
				JSONPath: "{.memberID}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MemberSpec](MemberType, &Member{})
	if err != nil {
		panic(err)
	}
}
