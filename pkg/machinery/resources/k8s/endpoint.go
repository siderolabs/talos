// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"net/netip"
	"sort"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EndpointType is type of Endpoint resource.
const EndpointType = resource.Type("Endpoints.kubernetes.talos.dev")

// ControlPlaneAPIServerEndpointsID is resource ID for kube-apiserver based Endpoints.
const ControlPlaneAPIServerEndpointsID = resource.ID("kube-apiserver")

// ControlPlaneDiscoveredEndpointsID is resource ID for cluster discovery based Endpoints.
const ControlPlaneDiscoveredEndpointsID = resource.ID("discovery")

// ControlPlaneKubernetesEndpointsID is resource ID for control plane endpoint-based Endpoints.
const ControlPlaneKubernetesEndpointsID = resource.ID("controlplane")

// Endpoint resource holds definition of rendered secrets.
type Endpoint = typed.Resource[EndpointSpec, EndpointExtension]

// EndpointSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type EndpointSpec struct {
	Addresses []netip.Addr `yaml:"addresses" protobuf:"1"`
}

// NewEndpoint initializes the Endpoint resource.
func NewEndpoint(namespace resource.Namespace, id resource.ID) *Endpoint {
	return typed.NewResource[EndpointSpec, EndpointExtension](
		resource.NewMetadata(namespace, EndpointType, id, resource.VersionUndefined),
		EndpointSpec{},
	)
}

// EndpointExtension provides auxiliary methods for Endpoint.
type EndpointExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EndpointExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
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

// EndpointList is a flattened list of endpoints.
type EndpointList []netip.Addr

// Merge endpoints from multiple Endpoint resources into a single list.
func (l EndpointList) Merge(endpoint *Endpoint) EndpointList {
	for _, ip := range endpoint.TypedSpec().Addresses {
		idx := sort.Search(len(l), func(i int) bool { return !l[i].Less(ip) })

		if idx < len(l) && l[idx].Compare(ip) == 0 {
			continue
		}

		l = append(l[:idx], append([]netip.Addr{ip}, l[idx:]...)...)
	}

	return l
}

// Strings returns a slice of formatted endpoints to string.
func (l EndpointList) Strings() []string {
	return xslices.Map(l, netip.Addr.String)
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EndpointSpec](EndpointType, &Endpoint{})
	if err != nil {
		panic(err)
	}
}
