// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// PeerSpecType is type of PeerSpec resource.
const PeerSpecType = resource.Type("KubeSpanPeerSpecs.kubespan.talos.dev")

// PeerSpec is produced from cluster.Affiliate which has KubeSpan information attached.
//
// PeerSpec is identified by the public key.
type PeerSpec struct {
	md   resource.Metadata
	spec PeerSpecSpec
}

// PeerSpecSpec describes PeerSpec state.
type PeerSpecSpec struct {
	Address    netaddr.IP         `yaml:"address"`
	AllowedIPs []netaddr.IPPrefix `yaml:"allowedIPs"`
	Endpoints  []netaddr.IPPort   `yaml:"endpoints"`
	Label      string             `yaml:"label"`
}

// NewPeerSpec initializes a PeerSpec resource.
func NewPeerSpec(namespace resource.Namespace, id resource.ID) *PeerSpec {
	r := &PeerSpec{
		md:   resource.NewMetadata(namespace, PeerSpecType, id, resource.VersionUndefined),
		spec: PeerSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *PeerSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *PeerSpec) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *PeerSpec) DeepCopy() resource.Resource {
	return &PeerSpec{
		md: r.md,
		spec: PeerSpecSpec{
			Address:    r.spec.Address,
			AllowedIPs: append([]netaddr.IPPrefix(nil), r.spec.AllowedIPs...),
			Endpoints:  append([]netaddr.IPPort(nil), r.spec.Endpoints...),
			Label:      r.spec.Label,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *PeerSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PeerSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Label",
				JSONPath: `{.label}`,
			},
			{
				Name:     "Endpoints",
				JSONPath: `{.endpoints}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *PeerSpec) TypedSpec() *PeerSpecSpec {
	return &r.spec
}
