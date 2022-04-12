// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"sort"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// EndpointType is type of Endpoint resource.
const EndpointType = resource.Type("Endpoints.kubernetes.talos.dev")

// ControlPlaneAPIServerEndpointsID is resource ID for kube-apiserver based Endpoints.
const ControlPlaneAPIServerEndpointsID = resource.ID("kube-apiserver")

// ControlPlaneDiscoveredEndpointsID is resource ID for cluster discovery based Endpoints.
const ControlPlaneDiscoveredEndpointsID = resource.ID("discovery")

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

// EndpointList is a flattened list of endpoints.
type EndpointList []netaddr.IP

// Merge endpoints from multiple Endpoint resources into a single list.
func (l EndpointList) Merge(endpoint *Endpoint) EndpointList {
	for _, ip := range endpoint.spec.Addresses {
		ip := ip

		idx := sort.Search(len(l), func(i int) bool { return !l[i].Less(ip) })

		if idx < len(l) && l[idx].Compare(ip) == 0 {
			continue
		}

		l = append(l[:idx], append([]netaddr.IP{ip}, l[idx:]...)...)
	}

	return l
}

// Strings returns a slice of formatted endpoints to string.
func (l EndpointList) Strings() []string {
	res := make([]string, len(l))

	for i := range l {
		res[i] = l[i].String()
	}

	return res
}
