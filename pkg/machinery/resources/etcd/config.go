// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ConfigType is type of Config resource.
const ConfigType = resource.Type("EtcdConfigs.etcd.talos.dev")

// ConfigID is resource ID for Config resource for etcd.
const ConfigID = resource.ID("etcd")

// Config resource holds status of rendered secrets.
type Config = typed.Resource[ConfigSpec, ConfigExtension]

// ConfigSpec describes (some) configuration settings of etcd.
//
//gotagsrewrite:gen
type ConfigSpec struct {
	AdvertiseValidSubnets   []string `yaml:"advertiseValidSubnets,omitempty" protobuf:"1"`
	AdvertiseExcludeSubnets []string `yaml:"advertiseExcludeSubnets" protobuf:"2"`

	ListenValidSubnets   []string `yaml:"listenValidSubnets,omitempty" protobuf:"5"`
	ListenExcludeSubnets []string `yaml:"listenExcludeSubnets" protobuf:"6"`

	Image string `yaml:"image" protobuf:"3"`

	ExtraArgs map[string]ArgValues `yaml:"extraArgs" protobuf:"4"`
}

// ArgValues represents values for a command line argument which can be specified multiple times.
//
//gotagsrewrite:gen
type ArgValues struct {
	Values []string `yaml:"values" protobuf:"1"`
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
