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

// KubePrismStatusesType is type of KubePrismStatuses resource.
const KubePrismStatusesType = resource.Type("KubePrismStatuses.kubernetes.talos.dev")

// KubePrismStatusesID the singleton balancer health data resource ID.
const KubePrismStatusesID = resource.ID("k8s-loadbalancer")

// KubePrismStatuses resource holds load balancer health data.
type KubePrismStatuses = typed.Resource[KubePrismStatusesSpec, KubePrismStatusesExtension]

// NewKubePrismStatuses initializes an KubePrismStatuses resource.
func NewKubePrismStatuses(namespace resource.Namespace, id resource.ID) *KubePrismStatuses {
	return typed.NewResource[KubePrismStatusesSpec, KubePrismStatusesExtension](
		resource.NewMetadata(namespace, KubePrismStatusesType, id, resource.VersionUndefined),
		KubePrismStatusesSpec{},
	)
}

// KubePrismStatusesSpec describes KubePrismStatuses data.
//
//gotagsrewrite:gen
type KubePrismStatusesSpec struct {
	Host    string `yaml:"host" protobuf:"1"`
	Healthy bool   `yaml:"healthy" protobuf:"2"`
}

// KubePrismStatusesExtension provides auxiliary methods for KubePrismStatuses.
type KubePrismStatusesExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubePrismStatusesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubePrismStatusesType,
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

	err := protobuf.RegisterDynamic[KubePrismStatusesSpec](KubePrismStatusesType, &KubePrismStatuses{})
	if err != nil {
		panic(err)
	}
}
