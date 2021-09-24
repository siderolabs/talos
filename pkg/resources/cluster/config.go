// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/resources/config"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("DiscoveryConfigs.cluster.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("cluster")

// Config resource holds KubeSpan configuration.
type Config struct {
	md   resource.Metadata
	spec ConfigSpec
}

// ConfigSpec describes KubeSpan configuration..
type ConfigSpec struct {
	DiscoveryEnabled          bool   `yaml:"discoveryEnabled"`
	RegistryKubernetesEnabled bool   `yaml:"registryKubernetesEnabled"`
	RegistryServiceEnabled    bool   `yaml:"registryServiceEnabled"`
	ServiceEndpoint           string `yaml:"serviceEndpoint"`
	ServiceEncryptionKey      []byte `yaml:"serviceEncryptionKey"`
	ServiceClusterID          string `yaml:"serviceClusterID"`
}

// NewConfig initializes a Config resource.
func NewConfig(namespace resource.Namespace, id resource.ID) *Config {
	r := &Config{
		md:   resource.NewMetadata(namespace, ConfigType, id, resource.VersionUndefined),
		spec: ConfigSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Config) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Config) Spec() interface{} {
	return r.spec
}

func (r *Config) String() string {
	return fmt.Sprintf("cluster.Config(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Config) DeepCopy() resource.Resource {
	return &Config{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Config) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *Config) TypedSpec() *ConfigSpec {
	return &r.spec
}
