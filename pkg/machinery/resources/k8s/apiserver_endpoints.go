// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// APIServerEndpointsType is type of APIServerEndpoints resource.
const APIServerEndpointsType = resource.Type("APIServerEndpoints.cluster.talos.dev")

// APIServerEndpointsID the singleton balancer data resource ID.
const APIServerEndpointsID = resource.ID("k8s-cluster")

// APIServerEndpoints resource holds endpoints data.
type APIServerEndpoints = typed.Resource[APIServerEndpointsSpec, APIServerEndpointsExtension]

// NewEndpoints initializes an APIServerEndpoints resource.
func NewEndpoints(namespace resource.Namespace, id resource.ID) *APIServerEndpoints {
	return typed.NewResource[APIServerEndpointsSpec, APIServerEndpointsExtension](
		resource.NewMetadata(namespace, APIServerEndpointsType, id, resource.VersionUndefined),
		APIServerEndpointsSpec{},
	)
}

// APIServerEndpointsSpec describes APIServerEndpoints configuration.
//
//gotagsrewrite:gen
type APIServerEndpointsSpec struct {
	Endpoints []APIServerEndpoint `yaml:"endpoints" protobuf:"1"`
}

// APIServerEndpoint holds data for control plane endpoint.
//
//gotagsrewrite:gen
type APIServerEndpoint struct {
	Host string `yaml:"host" protobuf:"1"`
	Port uint32 `yaml:"port" protobuf:"2"`
}

// String returns string representation of APIServerEndpoint.
func (e APIServerEndpoint) String() string {
	return fmt.Sprintf("host: %s, port: %d", e.Host, e.Port)
}

// APIServerEndpointsExtension provides auxiliary methods for APIServerEndpoints.
type APIServerEndpointsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (APIServerEndpointsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             APIServerEndpointsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Hosts",
				JSONPath: ".endpoints[*].host",
			},
			{
				Name:     "Ports",
				JSONPath: ".endpoints[*].port",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[APIServerEndpointsSpec](APIServerEndpointsType, &APIServerEndpoints{})
	if err != nil {
		panic(err)
	}
}
