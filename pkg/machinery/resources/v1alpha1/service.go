// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// ServiceType is type of Service resource.
const ServiceType = resource.Type("Services.v1alpha1.talos.dev")

// Service describes running service state.
type Service = typed.Resource[ServiceSpec, ServiceRD]

// ServiceSpec describe service state.
type ServiceSpec struct {
	Running bool `yaml:"running"`
	Healthy bool `yaml:"healthy"`
	Unknown bool `yaml:"unknown"`
}

// NewService initializes a Service resource.
func NewService(id resource.ID) *Service {
	return typed.NewResource[ServiceSpec, ServiceRD](
		resource.NewMetadata(NamespaceName, ServiceType, id, resource.VersionUndefined),
		ServiceSpec{},
	)
}

// DeepCopy implements the DeepCopyable interface.
func (s ServiceSpec) DeepCopy() ServiceSpec {
	return s
}

// ServiceRD provides auxiliary methods for Service.
type ServiceRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ServiceRD) ResourceDefinition(resource.Metadata, ServiceSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ServiceType,
		Aliases:          []resource.Type{"svc"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Running",
				JSONPath: "{.running}",
			},
			{
				Name:     "Healthy",
				JSONPath: "{.healthy}",
			},
			{
				Name:     "Health Unknown",
				JSONPath: "{.unknown}",
			},
		},
	}
}
