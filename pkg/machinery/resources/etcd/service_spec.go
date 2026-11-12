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

// ServiceSpecType is type of ServiceSpec resource.
const ServiceSpecType = resource.Type("EtcdServiceSpecs.etcd.talos.dev")

// ServiceSpecID is the ID of the singleton ServiceSpec resource.
const ServiceSpecID = resource.ID("etcd")

// ServiceSpec is the final specification the etcd service is running with.
//
// The resource is produced by the etcd service controller, and it exists while the etcd service is supposed to run.
type ServiceSpec = typed.Resource[ServiceSpecSpec, ServiceSpecExtension]

// ServiceSpecSpec describes the image and the arguments of the etcd service.
//
//gotagsrewrite:gen
type ServiceSpecSpec struct {
	// Image is the etcd container image reference.
	Image string `yaml:"image" protobuf:"1"`
	// Args is the full list of the etcd command line arguments.
	Args []string `yaml:"args" protobuf:"2"`
}

// NewServiceSpec initializes a ServiceSpec resource.
func NewServiceSpec() *ServiceSpec {
	return typed.NewResource[ServiceSpecSpec, ServiceSpecExtension](
		resource.NewMetadata(NamespaceName, ServiceSpecType, ServiceSpecID, resource.VersionUndefined),
		ServiceSpecSpec{},
	)
}

// ServiceSpecExtension provides auxiliary methods for ServiceSpec.
type ServiceSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ServiceSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ServiceSpecType,
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

	err := protobuf.RegisterDynamic[ServiceSpecSpec](ServiceSpecType, &ServiceSpec{})
	if err != nil {
		panic(err)
	}
}
