// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package siderolink contains SideroLink-related resources.
package siderolink

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

//go:generate deep-copy -type ConfigSpec -type TunnelSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// ConfigType is type of Config resource.
const ConfigType = resource.Type("SiderolinkConfigs.siderolink.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("siderolink")

// Config resource holds Siderolink configuration.
type Config = typed.Resource[ConfigSpec, ConfigExtension]

// ConfigSpec describes Siderolink configuration.
//
//gotagsrewrite:gen
type ConfigSpec struct {
	APIEndpoint string `yaml:"apiEndpoint" protobuf:"1"`
	Host        string `yaml:"host" protobuf:"2"`
	JoinToken   string `yaml:"joinToken" protobuf:"3"`
	Insecure    bool   `yaml:"insecure" protobuf:"4"`
	Tunnel      bool   `yaml:"tunnel" protobuf:"5"`
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
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "API Endpoint",
				JSONPath: `{.apiEndpoint}`,
			},
			{
				Name:     "Tunnel",
				JSONPath: `{.tunnel}`,
			},
		},
		Sensitivity: meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ConfigSpec](ConfigType, &Config{})
	if err != nil {
		panic(err)
	}
}
