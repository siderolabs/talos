// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MemberType is type of Member resource.
const MemberType = resource.Type("Members.cluster.talos.dev")

// Member resource contains information about discovered cluster members.
//
// Members are usually derived from Affiliates.
type Member = typed.Resource[MemberSpec, MemberExtension]

// MemberSpec describes Member state.
//
//gotagsrewrite:gen
type MemberSpec struct {
	NodeID          string       `yaml:"nodeId" protobuf:"1"`
	Addresses       []netip.Addr `yaml:"addresses" protobuf:"2"`
	Hostname        string       `yaml:"hostname" protobuf:"3"`
	MachineType     machine.Type `yaml:"machineType" protobuf:"4"`
	OperatingSystem string       `yaml:"operatingSystem" protobuf:"5"`
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
				Name:     "Hostname",
				JSONPath: `{.hostname}`,
			},
			{
				Name:     "Machine Type",
				JSONPath: `{.machineType}`,
			},
			{
				Name:     "OS",
				JSONPath: `{.operatingSystem}`,
			},
			{
				Name:     "Addresses",
				JSONPath: `{.addresses}`,
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
