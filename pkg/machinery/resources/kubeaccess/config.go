// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("KubernetesAccessConfigs.cluster.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("config")

// Config resource holds KubeSpan configuration.
type Config = typed.Resource[ConfigSpec, ConfigRD]

// ConfigSpec describes KubeSpan configuration..
type ConfigSpec struct {
	Enabled                     bool     `yaml:"enabled"`
	AllowedAPIRoles             []string `yaml:"allowedAPIRoles"`
	AllowedKubernetesNamespaces []string `yaml:"allowedKubernetesNamespaces"`
}

// DeepCopy generates a deep copy of ConfigSpec.
func (cs ConfigSpec) DeepCopy() ConfigSpec {
	cp := cs

	if cs.AllowedAPIRoles != nil {
		cp.AllowedAPIRoles = make([]string, len(cs.AllowedAPIRoles))
		copy(cp.AllowedAPIRoles, cs.AllowedAPIRoles)
	}

	if cs.AllowedKubernetesNamespaces != nil {
		cp.AllowedKubernetesNamespaces = make([]string, len(cs.AllowedKubernetesNamespaces))
		copy(cp.AllowedKubernetesNamespaces, cs.AllowedKubernetesNamespaces)
	}

	return cp
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
func (c ConfigRD) ResourceDefinition(resource.Metadata, ConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.NonSensitive,
	}
}
