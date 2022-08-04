// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("EtcdConfigs.etcd.talos.dev")

// ConfigID is resource ID for Config resource for etcd.
const ConfigID = resource.ID("etcd")

// Config resource holds status of rendered secrets.
type Config = typed.Resource[ConfigSpec, ConfigRD]

// ConfigSpec describes (some) configuration settings of etcd.
//
//gotagsrewrite:gen
type ConfigSpec struct {
	AdvertiseValidSubnets   []string `yaml:"advertiseValidSubnets,omitempty" protobuf:"1"`
	AdvertiseExcludeSubnets []string `yaml:"advertiseExcludeSubnets" protobuf:"2"`

	ListenValidSubnets   []string `yaml:"listenValidSubnets,omitempty" protobuf:"5"`
	ListenExcludeSubnets []string `yaml:"listenExcludeSubnets" protobuf:"6"`

	Image     string            `yaml:"image" protobuf:"3"`
	ExtraArgs map[string]string `yaml:"extraArgs" protobuf:"4"`
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
func (ConfigRD) ResourceDefinition(resource.Metadata, ConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Image",
				JSONPath: "{.image}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ConfigSpec](ConfigType, &Config{})
	if err != nil {
		panic(err)
	}
}
