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

// LoadBalancerStatusesType is type of LoadBalancerStatuses resource.
const LoadBalancerStatusesType = resource.Type("LoadBalancerStatuses.kubernetes.talos.dev")

// LoadBalancerStatusesID the singleton balancer health data resource ID.
const LoadBalancerStatusesID = resource.ID("k8s-loadbalancer")

// LoadBalancerStatuses resource holds load balancer health data.
type LoadBalancerStatuses = typed.Resource[LoadBalancerStatusesSpec, LoadBalancerStatusesExtension]

// NewLoadBalancerStatuses initializes an LoadBalancerStatuses resource.
func NewLoadBalancerStatuses(namespace resource.Namespace, id resource.ID) *LoadBalancerStatuses {
	return typed.NewResource[LoadBalancerStatusesSpec, LoadBalancerStatusesExtension](
		resource.NewMetadata(namespace, LoadBalancerStatusesType, id, resource.VersionUndefined),
		LoadBalancerStatusesSpec{},
	)
}

// LoadBalancerStatusesSpec describes LoadBalancerStatuses data.
//
//gotagsrewrite:gen
type LoadBalancerStatusesSpec struct {
	Host    string `yaml:"host" protobuf:"1"`
	Healthy bool   `yaml:"healthy" protobuf:"2"`
}

// LoadBalancerStatusesExtension provides auxiliary methods for LoadBalancerStatuses.
type LoadBalancerStatusesExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LoadBalancerStatusesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LoadBalancerStatusesType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "HOST",
				JSONPath: "{.host}",
			},
			{
				Name:     "HEALTHY",
				JSONPath: "{.healthy}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LoadBalancerStatusesSpec](LoadBalancerStatusesType, &LoadBalancerStatuses{})
	if err != nil {
		panic(err)
	}
}
