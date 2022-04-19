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
type OperatorSpecSpec struct {
	Operator  Operator `yaml:"operator"`
	LinkName  string   `yaml:"linkName"`
	RequireUp bool     `yaml:"requireUp"`

	DHCP4 DHCP4OperatorSpec `yaml:"dhcp4,omitempty"`
	DHCP6 DHCP6OperatorSpec `yaml:"dhcp6,omitempty"`
	VIP   VIPOperatorSpec   `yaml:"vip,omitempty"`

	ConfigLayer ConfigLayer `yaml:"layer"`
}

// DeepCopy generates a deep copy of OperatorSpecSpec.
func (spec OperatorSpecSpec) DeepCopy() OperatorSpecSpec {
	return spec
}

// DHCP4OperatorSpec describes DHCP4 operator options.
type DHCP4OperatorSpec struct {
	RouteMetric uint32 `yaml:"routeMetric"`
}

// DHCP6OperatorSpec describes DHCP6 operator options.
type DHCP6OperatorSpec struct {
	DUID        string `yaml:"DUID,omitempty"`
	RouteMetric uint32 `yaml:"routeMetric"`
}

// VIPOperatorSpec describes virtual IP operator options.
type VIPOperatorSpec struct {
	IP            netaddr.IP `yaml:"ip"`
	GratuitousARP bool       `yaml:"gratuitousARP"`

	EquinixMetal VIPEquinixMetalSpec `yaml:"equinixMetal,omitempty"`
	HCloud       VIPHCloudSpec       `yaml:"hcloud,omitempty"`
}

// VIPEquinixMetalSpec describes virtual (elastic) IP settings for Equinix Metal.
type VIPEquinixMetalSpec struct {
	ProjectID string `yaml:"projectID"`
	DeviceID  string `yaml:"deviceID"`
	APIToken  string `yaml:"apiToken"`
}

// VIPHCloudSpec describes virtual (elastic) IP settings for Hetzner Cloud.
type VIPHCloudSpec struct {
	DeviceID  int    `yaml:"deviceID"`
	NetworkID int    `yaml:"networkID"`
	APIToken  string `yaml:"apiToken"`
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
