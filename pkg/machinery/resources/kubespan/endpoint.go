// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// EndpointType is type of Endpoint resource.
const EndpointType = resource.Type("KubeSpanEndpoints.kubespan.talos.dev")

// Endpoint is produced from KubeSpanPeerStatuses by mapping back discovered endpoints to the affiliates.
//
// Endpoint is identified by the public key of the peer.
type Endpoint = typed.Resource[EndpointSpec, EndpointRD]

// EndpointSpec describes Endpoint state.
//
//gotagsrewrite:gen
type EndpointSpec struct {
	AffiliateID string         `yaml:"affiliateID" protobuf:"1"`
	Endpoint    netip.AddrPort `yaml:"endpoint" protobuf:"2"`
}

// NewEndpoint initializes a Endpoint resource.
func NewEndpoint(namespace resource.Namespace, id resource.ID) *Endpoint {
	return typed.NewResource[EndpointSpec, EndpointRD](
		resource.NewMetadata(namespace, EndpointType, id, resource.VersionUndefined),
		EndpointSpec{},
	)
}

// EndpointRD provides auxiliary methods for Endpoint.
type EndpointRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (EndpointRD) ResourceDefinition(resource.Metadata, EndpointSpec) meta.ResourceDefinitionSpec {
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

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EndpointSpec](EndpointType, &Endpoint{})
	if err != nil {
		panic(err)
	}
}
