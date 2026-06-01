// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("KubernetesAccessConfigs.cluster.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("config")

// Config resource holds KubeSpan configuration.
type Config = typed.Resource[ConfigSpec, ConfigExtension]

// ConfigSpec describes KubeSpan configuration..
//
//gotagsrewrite:gen
type ConfigSpec struct {
	Enabled                     bool     `yaml:"enabled" protobuf:"1"`
	AllowedAPIRoles             []string `yaml:"allowedAPIRoles" protobuf:"2"`
	AllowedKubernetesNamespaces []string `yaml:"allowedKubernetesNamespaces" protobuf:"3"`
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
	return typed.NewResource[ConfigSpec, ConfigExtension](
		resource.NewMetadata(namespace, ConfigType, id, resource.VersionUndefined),
		ConfigSpec{},
	)
}

// ConfigExtension provides auxiliary methods for Config.
type ConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (c ConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.NonSensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ConfigSpec](ConfigType, &Config{})
	if err != nil {
		panic(err)
	}
}
