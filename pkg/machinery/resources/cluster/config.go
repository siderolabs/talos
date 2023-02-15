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

// Config resource holds KubeSpan configuration.
type Config = typed.Resource[ConfigSpec, ConfigRD]

// ConfigSpec describes KubeSpan configuration.
//
//gotagsrewrite:gen
type ConfigSpec struct {
	DiscoveryEnabled          bool   `yaml:"discoveryEnabled" protobuf:"1"`
	RegistryKubernetesEnabled bool   `yaml:"registryKubernetesEnabled" protobuf:"2"`
	RegistryServiceEnabled    bool   `yaml:"registryServiceEnabled" protobuf:"3"`
	ServiceEndpoint           string `yaml:"serviceEndpoint" protobuf:"4"`
	ServiceEndpointInsecure   bool   `yaml:"serviceEndpointInsecure,omitempty" protobuf:"5"`
	ServiceEncryptionKey      []byte `yaml:"serviceEncryptionKey" protobuf:"6"`
	ServiceClusterID          string `yaml:"serviceClusterID" protobuf:"7"`
}

// NewConfig initializes a Config resource.
func NewConfig(namespace resource.Namespace, id resource.ID) *Config {
	return typed.NewResource[ConfigSpec, ConfigRD](
		resource.NewMetadata(namespace, ConfigType, id, resource.VersionUndefined),
		ConfigSpec{},
	)
}

// ConfigRD provides auxiliary methods for Config.
type ConfigRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (c ConfigRD) ResourceDefinition() meta.ResourceDefinitionSpec {
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
