// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// ServiceType is type of Service resource.
const ServiceType = resource.Type("Services.v1alpha1.talos.dev")

// Service describes running service state.
type Service struct {
	md   resource.Metadata
	spec ServiceSpec
}

// ServiceSpec describe service state.
type ServiceSpec struct {
	Running bool `yaml:"running"`
	Healthy bool `yaml:"healthy"`
}

// NewService initializes a Service resource.
func NewService(id resource.ID) *Service {
	r := &Service{
		md:   resource.NewMetadata(NamespaceName, ServiceType, id, resource.VersionUndefined),
		spec: ServiceSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Service) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Service) Spec() interface{} {
	return r.spec
}

func (r *Service) String() string {
	return fmt.Sprintf("v1alpha1.Service(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Service) DeepCopy() resource.Resource {
	return &Service{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Service) ResourceDefinition() meta.ResourceDefinitionSpec {
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
		},
	}
}

// SetRunning changes .spec.running.
func (r *Service) SetRunning(running bool) {
	r.spec.Running = running
}

// SetHealthy changes .spec.healthy.
func (r *Service) SetHealthy(healthy bool) {
	r.spec.Healthy = healthy
}

// Running returns .spec.running.
func (r *Service) Running() bool {
	return r.spec.Running
}

// Healthy returns .spec.healthy.
func (r *Service) Healthy() bool {
	return r.spec.Healthy
}
