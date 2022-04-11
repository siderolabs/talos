// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"fmt"

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
type Member struct{}

// MemberSpec describes Member state.
type MemberSpec struct {
	NodeID          string       `yaml:"nodeId"`
	Addresses       []netaddr.IP `yaml:"addresses"`
	Hostname        string       `yaml:"hostname"`
	MachineType     machine.Type `yaml:"machineType"`
	OperatingSystem string       `yaml:"operatingSystem"`
}

// NewMember initializes a Member resource.
func NewMember(namespace resource.Namespace, id resource.ID) *TypedResource[MemberSpec, Member] {
	return NewTypedResource[MemberSpec, Member](
		resource.NewMetadata(namespace, MemberType, id, resource.VersionUndefined),
		MemberSpec{},
	)
}

func (Member) String(md resource.Metadata, _ MemberSpec) string {
	return fmt.Sprintf("cluster.Member(%q)", md.ID())
}

// ResourceDefinition returns proper meta.ResourceDefinitionProvider for current type.
func (Member) ResourceDefinition(_ resource.Metadata, _ MemberSpec) meta.ResourceDefinitionSpec {
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
