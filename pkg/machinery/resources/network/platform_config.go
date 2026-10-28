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
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
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
//redactgen:gen
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
		// carries the link, operator and probe specs of the platform, secrets included
		Sensitivity: meta.Sensitive,
	}
}

// redactPlatformConfigValue redacts the secrets of a marshaled [PlatformConfigSpec].
//
// This is the shape of the value of the MetalNetworkPlatformConfig META key, which is why the
// redactor is registered here: the runtime resource package which holds MetaKey can't see the shape
// of the config, as it is the network resource package which depends on it and not the other way
// around.
//
// Most of what the config carries -- addresses, links, routes, hostnames -- is not a secret, and a
// config written by hand or by the dashboard holds nothing sensitive at all; only the embedded
// Wireguard, VIP and probe specs might. Such a value is handed back untouched, byte for byte, rather
// than re-marshaled, so that what the caller sees is exactly what was written.
func redactPlatformConfigValue(value string) (string, bool) {
	var spec PlatformConfigSpec

	if err := yaml.Unmarshal([]byte(value), &spec); err != nil {
		// the value is not a config we can look into, so assume the worst about it
		return constants.Redacted, true
	}

	if !hasSecretsPlatformConfigSpec(&spec) {
		return value, false
	}

	redactPlatformConfigSpec(&spec)

	redacted, err := yaml.Marshal(&spec)
	if err != nil {
		return constants.Redacted, true
	}

	return string(redacted), true
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[PlatformConfigSpec](PlatformConfigType, &PlatformConfig{})
	if err != nil {
		panic(err)
	}

	// the spec is sensitive, but it is safe to be read in the redacted form
	redactable.Register[PlatformConfigSpec, PlatformConfigExtension]()

	runtime.RegisterMetaKeyValueRedactor(runtime.MetaKeyTagToID(metaconsts.MetalNetworkPlatformConfig), redactPlatformConfigValue)
}
