// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/core"
)

// ServiceType is type of Service resource.
const ServiceType = resource.Type("v1alpha1/service")

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

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (r *Service) ResourceDefinition() core.ResourceDefinitionSpec {
	return core.ResourceDefinitionSpec{
		Type:             ServiceType,
		Aliases:          []resource.Type{"svc", "services", "service"},
		DefaultNamespace: NamespaceName,
	}
}

// SetRunning changes .spec.running.
func (r *Service) SetRunning(running bool) {
	r.spec.Running = true
}
