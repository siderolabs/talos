// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"bytes"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// PlatformConfigType is type of PlatformConfig resource.
const PlatformConfigType = resource.Type("PlatformConfigs.net.talos.dev")

// PlatformConfig resource holds DNS resolver info.
type PlatformConfig = typed.Resource[PlatformConfigSpec, PlatformConfigExtension]

// PlatformConfigActiveID is the ID of instance containing active config.
const PlatformConfigActiveID resource.ID = "active"

// PlatformConfigCachedID is the ID of instance containing cached (persisted) config.
const PlatformConfigCachedID resource.ID = "cached"

// PlatformConfigSpec describes platform network configuration.
//
// This structure is marshaled to STATE partition to persist cached network configuration across
// reboots.
//
//gotagsrewrite:gen
type PlatformConfigSpec struct {
	Addresses []AddressSpecSpec `yaml:"addresses" protobuf:"1"`
	Links     []LinkSpecSpec    `yaml:"links" protobuf:"2"`
	Routes    []RouteSpecSpec   `yaml:"routes" protobuf:"3"`

	Hostnames   []HostnameSpecSpec   `yaml:"hostnames" protobuf:"4"`
	Resolvers   []ResolverSpecSpec   `yaml:"resolvers" protobuf:"5"`
	TimeServers []TimeServerSpecSpec `yaml:"timeServers" protobuf:"6"`

	Operators []OperatorSpecSpec `yaml:"operators" protobuf:"7"`

	ExternalIPs []netip.Addr `yaml:"externalIPs" protobuf:"8"`

	Probes []ProbeSpecSpec `yaml:"probes,omitempty" protobuf:"9"`

	Metadata *runtime.PlatformMetadataSpec `yaml:"metadata,omitempty" protobuf:"10"`
}

// Equal compares two platform network configurations.
func (p *PlatformConfigSpec) Equal(other *PlatformConfigSpec) bool {
	// we will compare by marshaling to YAML
	// and then comparing the bytes
	// this is not the most efficient way to do this,
	// but it will handle omitting empty fields
	m1, err1 := yaml.Marshal(p)

	m2, err2 := yaml.Marshal(other)
	if err1 != nil || err2 != nil {
		return false
	}

	return bytes.Equal(m1, m2)
}

// NewPlatformConfig initializes a PlatformConfig resource.
func NewPlatformConfig(namespace resource.Namespace, id resource.ID) *PlatformConfig {
	return typed.NewResource[PlatformConfigSpec, PlatformConfigExtension](
		resource.NewMetadata(namespace, PlatformConfigType, id, resource.VersionUndefined),
		PlatformConfigSpec{},
	)
}

// PlatformConfigExtension provides auxiliary methods for PlatformConfig.
type PlatformConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PlatformConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PlatformConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[PlatformConfigSpec](PlatformConfigType, &PlatformConfig{})
	if err != nil {
		panic(err)
	}
}
