// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"inet.af/netaddr"
)

// OperatorSpecType is type of OperatorSpec resource.
const OperatorSpecType = resource.Type("OperatorSpecs.net.talos.dev")

// OperatorSpec resource holds DNS resolver info.
type OperatorSpec = typed.Resource[OperatorSpecSpec, OperatorSpecRD]

// OperatorSpecSpec describes DNS resolvers.
//
//gotagsrewrite:gen
type OperatorSpecSpec struct {
	Operator  Operator `yaml:"operator" protobuf:"1"`
	LinkName  string   `yaml:"linkName" protobuf:"2"`
	RequireUp bool     `yaml:"requireUp" protobuf:"3"`

	DHCP4 DHCP4OperatorSpec `yaml:"dhcp4,omitempty" protobuf:"4"`
	DHCP6 DHCP6OperatorSpec `yaml:"dhcp6,omitempty" protobuf:"5"`
	VIP   VIPOperatorSpec   `yaml:"vip,omitempty" protobuf:"6"`

	ConfigLayer ConfigLayer `yaml:"layer" protobuf:"7"`
}

// DHCP4OperatorSpec describes DHCP4 operator options.
//
//gotagsrewrite:gen
type DHCP4OperatorSpec struct {
	RouteMetric uint32 `yaml:"routeMetric" protobuf:"1"`
}

// DHCP6OperatorSpec describes DHCP6 operator options.
//
//gotagsrewrite:gen
type DHCP6OperatorSpec struct {
	DUID        string `yaml:"DUID,omitempty" protobuf:"1"`
	RouteMetric uint32 `yaml:"routeMetric" protobuf:"2"`
}

// VIPOperatorSpec describes virtual IP operator options.
//
//gotagsrewrite:gen
type VIPOperatorSpec struct {
	IP            netaddr.IP `yaml:"ip" protobuf:"1"`
	GratuitousARP bool       `yaml:"gratuitousARP" protobuf:"2"`

	EquinixMetal VIPEquinixMetalSpec `yaml:"equinixMetal,omitempty" protobuf:"3"`
	HCloud       VIPHCloudSpec       `yaml:"hcloud,omitempty" protobuf:"4"`
}

// VIPEquinixMetalSpec describes virtual (elastic) IP settings for Equinix Metal.
//
//gotagsrewrite:gen
type VIPEquinixMetalSpec struct {
	ProjectID string `yaml:"projectID" protobuf:"1"`
	DeviceID  string `yaml:"deviceID" protobuf:"2"`
	APIToken  string `yaml:"apiToken" protobuf:"3"`
}

// VIPHCloudSpec describes virtual (elastic) IP settings for Hetzner Cloud.
//
//gotagsrewrite:gen
type VIPHCloudSpec struct {
	DeviceID  int    `yaml:"deviceID" protobuf:"1"`
	NetworkID int    `yaml:"networkID" protobuf:"2"`
	APIToken  string `yaml:"apiToken" protobuf:"3"`
}

// NewOperatorSpec initializes a OperatorSpec resource.
func NewOperatorSpec(namespace resource.Namespace, id resource.ID) *OperatorSpec {
	return typed.NewResource[OperatorSpecSpec, OperatorSpecRD](
		resource.NewMetadata(namespace, OperatorSpecType, id, resource.VersionUndefined),
		OperatorSpecSpec{},
	)
}

// OperatorSpecRD provides auxiliary methods for OperatorSpec.
type OperatorSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (OperatorSpecRD) ResourceDefinition(resource.Metadata, OperatorSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             OperatorSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}
