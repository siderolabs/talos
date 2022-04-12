// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// MemberType is type of Member resource.
const MemberType = resource.Type("Members.cluster.talos.dev")

// Member resource contains information about discovered cluster members.
//
// Members are usually derived from Affiliates.
type Member struct {
	md   resource.Metadata
	spec MemberSpec
}

// MemberSpec describes Member state.
type MemberSpec struct {
	NodeID          string       `yaml:"nodeId"`
	Addresses       []netaddr.IP `yaml:"addresses"`
	Hostname        string       `yaml:"hostname"`
	MachineType     machine.Type `yaml:"machineType"`
	OperatingSystem string       `yaml:"operatingSystem"`
}

// NewMember initializes a Member resource.
func NewMember(namespace resource.Namespace, id resource.ID) *Member {
	r := &Member{
		md:   resource.NewMetadata(namespace, MemberType, id, resource.VersionUndefined),
		spec: MemberSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Member) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Member) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *Member) DeepCopy() resource.Resource {
	return &Member{
		md: r.md,
		spec: MemberSpec{
			NodeID:          r.spec.NodeID,
			Addresses:       append([]netaddr.IP(nil), r.spec.Addresses...),
			Hostname:        r.spec.Hostname,
			MachineType:     r.spec.MachineType,
			OperatingSystem: r.spec.OperatingSystem,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Member) ResourceDefinition() meta.ResourceDefinitionSpec {
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

// TypedSpec allows to access the Spec with the proper type.
func (r *Member) TypedSpec() *MemberSpec {
	return &r.spec
}
