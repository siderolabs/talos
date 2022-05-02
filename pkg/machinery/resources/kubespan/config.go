// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("KubeSpanConfigs.kubespan.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("kubespan")

// Config resource holds KubeSpan configuration.
type Config = typed.Resource[ConfigSpec, ConfigRD]

// ConfigSpec describes KubeSpan configuration..
type ConfigSpec struct {
	Enabled      bool   `yaml:"enabled"`
	ClusterID    string `yaml:"clusterId"`
	SharedSecret string `yaml:"sharedSecret"`
	// Force routing via KubeSpan even if the peer connection is not up.
	ForceRouting bool `yaml:"forceRouting"`
}

// DeepCopy implements typed.DeepCopyable interface.
func (spec ConfigSpec) DeepCopy() ConfigSpec { return spec }

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
func (ConfigRD) ResourceDefinition(resource.Metadata, ConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}
