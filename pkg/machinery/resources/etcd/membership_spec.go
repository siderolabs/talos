// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MembershipSpecType is type of MembershipSpec resource.
const MembershipSpecType = resource.Type("EtcdMembershipSpecs.etcd.talos.dev")

// MembershipSpecID is the ID of the singleton MembershipSpec resource.
const MembershipSpecID = resource.ID("etcd")

// MembershipSpec describes how the etcd member on this machine should enter the etcd cluster.
//
// The resource is the (only) configuration of the etcd membership controller: it exists once everything
// required to decide on the etcd cluster membership is known, and it is gone if etcd is not configured.
type MembershipSpec = typed.Resource[MembershipSpecSpec, MembershipSpecExtension]

// MembershipSpecSpec describes the etcd member with respect to the etcd cluster membership.
//
//gotagsrewrite:gen
type MembershipSpecSpec struct {
	// Name is the name of the etcd member.
	Name string `yaml:"name" protobuf:"1"`
	// AdvertisedAddresses is the list of addresses the etcd member advertises to the cluster.
	AdvertisedAddresses []netip.Addr `yaml:"advertisedAddresses" protobuf:"2"`
	// ManualInitialCluster is set if the etcd initial cluster is supplied in the etcd extra arguments,
	// so the cluster membership is managed outside of Talos.
	ManualInitialCluster bool `yaml:"manualInitialCluster" protobuf:"3"`
	// InitNode is set if the machine is of type init.
	InitNode bool `yaml:"initNode" protobuf:"4"`
	// AutoBootstrap is set if the machine should bootstrap the etcd cluster on its own if it has no etcd data.
	AutoBootstrap bool `yaml:"autoBootstrap" protobuf:"5"`
}

// NewMembershipSpec initializes a MembershipSpec resource.
func NewMembershipSpec() *MembershipSpec {
	return typed.NewResource[MembershipSpecSpec, MembershipSpecExtension](
		resource.NewMetadata(NamespaceName, MembershipSpecType, MembershipSpecID, resource.VersionUndefined),
		MembershipSpecSpec{},
	)
}

// MembershipSpecExtension provides auxiliary methods for MembershipSpec.
type MembershipSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MembershipSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MembershipSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: "{.name}",
			},
			{
				Name:     "Advertised Addresses",
				JSONPath: "{.advertisedAddresses}",
			},
			{
				Name:     "Auto Bootstrap",
				JSONPath: "{.autoBootstrap}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MembershipSpecSpec](MembershipSpecType, &MembershipSpec{})
	if err != nil {
		panic(err)
	}
}
