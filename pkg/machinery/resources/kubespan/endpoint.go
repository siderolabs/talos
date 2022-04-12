// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// EndpointType is type of Endpoint resource.
const EndpointType = resource.Type("KubeSpanEndpoints.kubespan.talos.dev")

// Endpoint is produced from KubeSpanPeerStatuses by mapping back discovered endpoints to the affiliates.
//
// Endpoint is identified by the public key of the peer.
type Endpoint struct {
	md   resource.Metadata
	spec EndpointSpec
}

// EndpointSpec describes Endpoint state.
type EndpointSpec struct {
	AffiliateID string         `yaml:"affiliateID"`
	Endpoint    netaddr.IPPort `yaml:"endpoint"`
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

// DeepCopy implements resource.Resource.
func (r *Endpoint) DeepCopy() resource.Resource {
	return &Endpoint{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Endpoint) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EndpointType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Endpoint",
				JSONPath: `{.endpoint}`,
			},
			{
				Name:     "Affiliate ID",
				JSONPath: `{.affiliateID}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *Endpoint) TypedSpec() *EndpointSpec {
	return &r.spec
}
