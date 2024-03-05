// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// TunnelType is type of Tunnel resource.
const TunnelType = resource.Type("SiderolinkTunnels.siderolink.talos.dev")

// TunnelID the singleton tunnel resource ID.
const TunnelID = resource.ID("siderolink-tunnel")

// Tunnel resource holds Siderolink GRPC Tunnel configuration.
type Tunnel = typed.Resource[TunnelSpec, TunnelExtension]

// TunnelSpec describes Siderolink GRPC Tunnel configuration.
//
//gotagsrewrite:gen
type TunnelSpec struct {
	// APIEndpoint is the Siderolink WireGuard over GRPC endpoint.
	APIEndpoint string `yaml:"apiEndpoint" protobuf:"1"`
	// LinkName is the name to use for WireGuard tunnel.
	LinkName string `yaml:"linkName" protobuf:"2"`
	// MTU is the maximum transmission unit for the tunnel.
	MTU int `yaml:"mtu" protobuf:"3"`
	// NodeAddress is the virtual address of our node. It's used to identify our node in the WireGuard GRPC streamer.
	// It's not the address of the actual WireGuard interface.
	NodeAddress netip.AddrPort `yaml:"nodeAddress" protobuf:"4"`
}

// NewTunnel initializes a Config resource.
func NewTunnel() *Tunnel {
	return typed.NewResource[TunnelSpec, TunnelExtension](
		resource.NewMetadata(config.NamespaceName, TunnelType, TunnelID, resource.VersionUndefined),
		TunnelSpec{},
	)
}

// TunnelExtension provides auxiliary methods for Tunnel.
type TunnelExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (TunnelExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TunnelType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "API Endpoint",
				JSONPath: `{.apiEndpoint}`,
			},
			{
				Name:     "Interface name",
				JSONPath: `{.ifaceName}`,
			},
			{
				Name:     "MTU",
				JSONPath: `{.mtu}`,
			},
			{
				Name:     "Node address",
				JSONPath: `{.nodeAddress}`,
			},
		},
		Sensitivity: meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[TunnelSpec](TunnelType, &Tunnel{})
	if err != nil {
		panic(err)
	}
}
