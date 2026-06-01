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

// KubePrismEndpointsType is type of KubePrismEndpoints resource.
const KubePrismEndpointsType = resource.Type("KubePrismEndpoints.kubernetes.talos.dev")

// KubePrismEndpointsID the singleton balancer data resource ID.
const KubePrismEndpointsID = resource.ID("k8s-cluster")

// KubePrismEndpoints resource holds endpoints data.
type KubePrismEndpoints = typed.Resource[KubePrismEndpointsSpec, KubePrismEndpointsExtension]

// NewKubePrismEndpoints initializes an KubePrismEndpoints resource.
func NewKubePrismEndpoints(namespace resource.Namespace, id resource.ID) *KubePrismEndpoints {
	return typed.NewResource[KubePrismEndpointsSpec, KubePrismEndpointsExtension](
		resource.NewMetadata(namespace, KubePrismEndpointsType, id, resource.VersionUndefined),
		KubePrismEndpointsSpec{},
	)
}

// KubePrismEndpointsSpec describes KubePrismEndpoints configuration.
//
//gotagsrewrite:gen
type KubePrismEndpointsSpec struct {
	Endpoints []KubePrismEndpoint `yaml:"endpoints" protobuf:"1"`
}

// KubePrismEndpoint holds data for control plane endpoint.
//
//gotagsrewrite:gen
type KubePrismEndpoint struct {
	Host string `yaml:"host" protobuf:"1"`
	Port uint32 `yaml:"port" protobuf:"2"`
}

// String returns string representation of KubePrismEndpoint.
func (e KubePrismEndpoint) String() string {
	return fmt.Sprintf("host: %s, port: %d", e.Host, e.Port)
}

// KubePrismEndpointsExtension provides auxiliary methods for KubePrismEndpoints.
type KubePrismEndpointsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubePrismEndpointsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubePrismEndpointsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Hosts",
				JSONPath: "{.endpoints[*].host}",
			},
			{
				Name:     "Ports",
				JSONPath: "{.endpoints[*].port}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubePrismEndpointsSpec](KubePrismEndpointsType, &KubePrismEndpoints{})
	if err != nil {
		panic(err)
	}
}
