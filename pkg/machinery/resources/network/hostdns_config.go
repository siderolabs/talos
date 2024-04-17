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

// HostDNSConfigType is type of HostDNSConfig resource.
const HostDNSConfigType = resource.Type("HostDNSConfigs.net.talos.dev")

// HostDNSConfig resource holds host DNS config.
type HostDNSConfig = typed.Resource[HostDNSConfigSpec, HostDNSConfigExtension]

// HostDNSConfigID is the singleton ID for HostDNSConfig.
const HostDNSConfigID resource.ID = "config"

// HostDNSConfigSpec describes host DNS config.
//
//gotagsrewrite:gen
type HostDNSConfigSpec struct {
	Enabled               bool             `yaml:"enabled" protobuf:"1"`
	ListenAddresses       []netip.AddrPort `yaml:"listenAddresses,omitempty" protobuf:"2"`
	ServiceHostDNSAddress netip.Addr       `yaml:"serviceHostDNSAddress,omitempty" protobuf:"3"`
	ResolveMemberNames    bool             `yaml:"resolveMemberNames,omitempty" protobuf:"4"`
}

// NewHostDNSConfig initializes a HostDNSConfig resource.
func NewHostDNSConfig(id resource.ID) *HostDNSConfig {
	return typed.NewResource[HostDNSConfigSpec, HostDNSConfigExtension](
		resource.NewMetadata(NamespaceName, HostDNSConfigType, id, resource.VersionUndefined),
		HostDNSConfigSpec{},
	)
}

// HostDNSConfigExtension provides auxiliary methods for HostDNSConfig.
type HostDNSConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (HostDNSConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             HostDNSConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Enabled",
				JSONPath: "{.enabled}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[HostDNSConfigSpec](HostDNSConfigType, &HostDNSConfig{})
	if err != nil {
		panic(err)
	}
}
