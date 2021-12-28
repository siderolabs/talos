// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// OperatorSpecType is type of OperatorSpec resource.
const OperatorSpecType = resource.Type("OperatorSpecs.net.talos.dev")

// OperatorSpec resource holds DNS resolver info.
type OperatorSpec struct {
	md   resource.Metadata
	spec OperatorSpecSpec
}

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

// DHCP4OperatorSpec describes DHCP4 operator options.
type DHCP4OperatorSpec struct {
	RouteMetric uint32 `yaml:"routeMetric"`
}

// DHCP6OperatorSpec describes DHCP6 operator options.
type DHCP6OperatorSpec struct {
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
	r := &OperatorSpec{
		md:   resource.NewMetadata(namespace, OperatorSpecType, id, resource.VersionUndefined),
		spec: OperatorSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *OperatorSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *OperatorSpec) Spec() interface{} {
	return r.spec
}

func (r *OperatorSpec) String() string {
	return fmt.Sprintf("network.OperatorSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *OperatorSpec) DeepCopy() resource.Resource {
	return &OperatorSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *OperatorSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             OperatorSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *OperatorSpec) TypedSpec() *OperatorSpecSpec {
	return &r.spec
}
