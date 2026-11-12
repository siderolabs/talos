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

// MembershipStatusType is type of MembershipStatus resource.
const MembershipStatusType = resource.Type("EtcdMembershipStatuses.etcd.talos.dev")

// MembershipStatusID is the ID of the singleton MembershipStatus resource.
const MembershipStatusID = resource.ID("etcd")

// MembershipStatus describes the status of the etcd member on this machine with respect to the etcd cluster.
type MembershipStatus = typed.Resource[MembershipStatusSpec, MembershipStatusExtension]

// MembershipStatusSpec describes the status of the etcd cluster membership.
//
//gotagsrewrite:gen
type MembershipStatusSpec struct {
	// Phase is the current phase of the etcd membership.
	Phase MembershipPhase `yaml:"phase" protobuf:"1"`
	// MemberID is the etcd member ID of this machine, if it is known.
	MemberID string `yaml:"memberID,omitempty" protobuf:"2"`
	// Error is the last error encountered while driving the membership to the desired state.
	Error string `yaml:"error,omitempty" protobuf:"3"`
}

// NewMembershipStatus initializes a MembershipStatus resource.
func NewMembershipStatus() *MembershipStatus {
	return typed.NewResource[MembershipStatusSpec, MembershipStatusExtension](
		resource.NewMetadata(NamespaceName, MembershipStatusType, MembershipStatusID, resource.VersionUndefined),
		MembershipStatusSpec{},
	)
}

// MembershipStatusExtension provides auxiliary methods for MembershipStatus.
type MembershipStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MembershipStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MembershipStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Phase",
				JSONPath: "{.phase}",
			},
			{
				Name:     "Member ID",
				JSONPath: "{.memberID}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MembershipStatusSpec](MembershipStatusType, &MembershipStatus{})
	if err != nil {
		panic(err)
	}
}
