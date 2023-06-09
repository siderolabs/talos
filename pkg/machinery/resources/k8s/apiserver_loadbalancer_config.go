// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LoadBalancerConfigType is type of LoadBalancerConfig resource.
const LoadBalancerConfigType = resource.Type("LoadBalancerConfigs.kubernetes.talos.dev")

// LoadBalancerConfigID the singleton config resource ID.
const LoadBalancerConfigID = resource.ID("k8s-loadbalancer-config")

// LoadBalancerConfig resource holds load balancer health data.
type LoadBalancerConfig = typed.Resource[LoadBalancerConfigSpec, LoadBalancerConfigExtension]

// NewLoadBalancerConfig initializes an LoadBalancerConfig resource.
func NewLoadBalancerConfig(namespace resource.Namespace, id resource.ID) *LoadBalancerConfig {
	return typed.NewResource[LoadBalancerConfigSpec, LoadBalancerConfigExtension](
		resource.NewMetadata(namespace, LoadBalancerConfigType, id, resource.VersionUndefined),
		LoadBalancerConfigSpec{},
	)
}

// LoadBalancerConfigSpec describes LoadBalancerConfig data.
//
//gotagsrewrite:gen
type LoadBalancerConfigSpec struct {
	Host      string              `yaml:"host" protobuf:"1"`
	Port      int                 `yaml:"port" protobuf:"2"`
	Endpoints []APIServerEndpoint `yaml:"endpoints" protobuf:"3"`
}

// LoadBalancerConfigExtension provides auxiliary methods for LoadBalancerConfig.
type LoadBalancerConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LoadBalancerConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LoadBalancerConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Host",
				JSONPath: "{.host}",
			},
			{
				Name:     "Port",
				JSONPath: "{.port}",
			},
			{
				Name:     "Endpoints",
				JSONPath: "{.endpoints}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LoadBalancerConfigSpec](LoadBalancerConfigType, &LoadBalancerConfig{})
	if err != nil {
		panic(err)
	}
}
