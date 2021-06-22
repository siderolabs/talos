// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// EndpointType is type of Endpoint resource.
const EndpointType = resource.Type("Endpoints.kubernetes.talos.dev")

// ControlPlaneEndpointsID is resource ID for controlplane Endpoint.
const ControlPlaneEndpointsID = resource.ID("controlplane")

// Endpoint resource holds definition of rendered secrets.
type Endpoint struct {
	md   resource.Metadata
	spec EndpointSpec
}

// EndpointSpec describes status of rendered secrets.
type EndpointSpec struct {
	Addresses []netaddr.IP `yaml:"addresses"`
}

// NewEndpoint initializes a Endpoint resource.
func NewEndpoint(namespace resource.Namespace, id resource.ID) *Endpoint {
	r := &Endpoint{
		md:   resource.NewMetadata(namespace, EndpointType, id, resource.VersionUndefined),
		spec: EndpointSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Endpoint) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Endpoint) Spec() interface{} {
	return r.spec
}

func (r *Endpoint) String() string {
	return fmt.Sprintf("k8s.Endpoint(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Endpoint) DeepCopy() resource.Resource {
	return &Endpoint{
		md: r.md,
		spec: EndpointSpec{
			Addresses: append([]netaddr.IP(nil), r.spec.Addresses...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Endpoint) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EndpointType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Addresses",
				JSONPath: "{.addresses}",
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *Endpoint) TypedSpec() *EndpointSpec {
	return &r.spec
}
