// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// StaticHostType is type of StaticHost resource.
const StaticHostType = resource.Type("StaticHosts.net.talos.dev")

// StaticHost resource holds a static host entry resolved by the in-process DNS server.
//
// The resource ID is the host name (alias). The spec contains the addresses
// this name resolves to.
type StaticHost = typed.Resource[StaticHostSpec, StaticHostExtension]

// StaticHostSpec describes addresses for a static host name.
//
//gotagsrewrite:gen
type StaticHostSpec struct {
	Addresses []netip.Addr `yaml:"addresses" protobuf:"1"`
}

// NewStaticHost initializes a StaticHost resource.
func NewStaticHost(namespace resource.Namespace, id resource.ID) *StaticHost {
	return typed.NewResource[StaticHostSpec, StaticHostExtension](
		resource.NewMetadata(namespace, StaticHostType, id, resource.VersionUndefined),
		StaticHostSpec{},
	)
}

// StaticHostExtension provides auxiliary methods for StaticHost.
type StaticHostExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (StaticHostExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StaticHostType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Addresses",
				JSONPath: "{.addresses}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[StaticHostSpec](StaticHostType, &StaticHost{})
	if err != nil {
		panic(err)
	}
}
