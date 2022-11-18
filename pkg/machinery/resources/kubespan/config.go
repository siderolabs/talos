// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

//nolint:lll
//go:generate deep-copy -type ConfigSpec -type EndpointSpec -type IdentitySpec -type PeerSpecSpec -type PeerStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// ConfigType is type of Config resource.
const ConfigType = resource.Type("KubeSpanConfigs.kubespan.talos.dev")

// ConfigID the singleton config resource ID.
const ConfigID = resource.ID("kubespan")

// Config resource holds KubeSpan configuration.
type Config = typed.Resource[ConfigSpec, ConfigRD]

// ConfigSpec describes KubeSpan configuration..
//
//gotagsrewrite:gen
type ConfigSpec struct {
	Enabled      bool   `yaml:"enabled" protobuf:"1"`
	ClusterID    string `yaml:"clusterId" protobuf:"2"`
	SharedSecret string `yaml:"sharedSecret" protobuf:"3"`
	// Force routing via KubeSpan even if the peer connection is not up.
	ForceRouting bool `yaml:"forceRouting" protobuf:"4"`
	// Advertise Kubernetes pod networks or skip it completely.
	AdvertiseKubernetesNetworks bool `yaml:"advertiseKubernetesNetworks" protobuf:"5"`
	// Force kubeSpan MTU size.
	MTU uint32 `yaml:"mtu,omitempty" protobuf:"6"`
	// Allowed list of node endpoints.
	FilterEndpoints []string `yaml:"filterEndpoints,omitempty" protobuf:"7"`
	// FilterNativeRoute skip destination CIDR list.
	FilterNativeRoute []string `yaml:"filterNativeRoute,omitempty" protobuf:"8"`
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
