// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("DiscoveryConfigs.cluster.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("cluster")

// ConfigSpec describes KubeSpan configuration..
type ConfigSpec struct {
	DiscoveryEnabled          bool   `yaml:"discoveryEnabled"`
	RegistryKubernetesEnabled bool   `yaml:"registryKubernetesEnabled"`
	RegistryServiceEnabled    bool   `yaml:"registryServiceEnabled"`
	ServiceEndpoint           string `yaml:"serviceEndpoint"`
	ServiceEndpointInsecure   bool   `yaml:"serviceEndpointInsecure,omitempty"`
	ServiceEncryptionKey      []byte `yaml:"serviceEncryptionKey"`
	ServiceClusterID          string `yaml:"serviceClusterID"`
}

// NewConfig initializes a Config resource.
func NewConfig(namespace resource.Namespace, id resource.ID) *TypedResource[ConfigSpec, Config] {
	return NewTypedResource[ConfigSpec, Config](
		resource.NewMetadata(namespace, ConfigType, id, resource.VersionUndefined),
		ConfigSpec{},
	)
}

// Config resource holds KubeSpan configuration.
type Config struct{}

func (Config) String(md resource.Metadata, _ ConfigSpec) string {
	return fmt.Sprintf("cluster.Config(%q)", md.ID())
}

// ResourceDefinition returns proper meta.ResourceDefinitionProvider for current type.
func (Config) ResourceDefinition(md resource.Metadata, _ ConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}
