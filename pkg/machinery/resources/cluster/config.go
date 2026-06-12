// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("DiscoveryConfigs.cluster.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("cluster")

// Config resource holds Discovery configuration.
type Config = typed.Resource[ConfigSpec, ConfigExtension]

// ConfigSpec describes Discovery configuration.
//
//gotagsrewrite:gen
type ConfigSpec struct {
	// Deprecated: use ServiceEndpoints instead (configured via DiscoveryServiceConfig documents)
	DiscoveryEnabled bool `yaml:"discoveryEnabled" protobuf:"1"`

	RegistryKubernetesEnabled bool `yaml:"registryKubernetesEnabled" protobuf:"2"`

	// Deprecated: enabled via DiscoveryServiceConfig documents instead.
	RegistryServiceEnabled bool `yaml:"registryServiceEnabled" protobuf:"3"`
	// Deprecated: use ServiceEndpoints instead
	ServiceEndpoint string `yaml:"serviceEndpoint" protobuf:"4"`
	// Deprecated: use ServiceEndpoints instead
	ServiceEndpointInsecure bool `yaml:"serviceEndpointInsecure,omitempty" protobuf:"5"`

	ServiceEncryptionKey []byte            `yaml:"serviceEncryptionKey" protobuf:"6"`
	ServiceClusterID     string            `yaml:"serviceClusterID" protobuf:"7"`
	ServiceEndpoints     []ServiceEndpoint `yaml:"serviceEndpoints" protobuf:"8"`
}

// ServiceEndpoint describes a service endpoint for discovery.
//
//gotagsrewrite:gen
type ServiceEndpoint struct {
	Name     string `yaml:"name" protobuf:"1"`
	Endpoint string `yaml:"endpoint" protobuf:"2"`
	Insecure bool   `yaml:"insecure" protobuf:"3"`
}

// NewConfig initializes a Config resource.
func NewConfig(namespace resource.Namespace, id resource.ID) *Config {
	return typed.NewResource[ConfigSpec, ConfigExtension](
		resource.NewMetadata(namespace, ConfigType, id, resource.VersionUndefined),
		ConfigSpec{},
	)
}

// ConfigExtension provides auxiliary methods for Config.
type ConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ConfigSpec](ConfigType, &Config{})
	if err != nil {
		panic(err)
	}
}
