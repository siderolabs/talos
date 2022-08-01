// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// SpecType is type of Spec resource.
const SpecType = resource.Type("EtcdSpecs.etcd.talos.dev")

// SpecID is resource ID for Spec resource for etcd.
const SpecID = resource.ID("etcd")

// Spec resource holds status of rendered secrets.
type Spec = typed.Resource[SpecSpec, SpecRD]

// SpecSpec describes (some) Specuration settings of etcd.
//
//gotagsrewrite:gen
type SpecSpec struct {
	Name              string            `yaml:"name" protobuf:"1"`
	AdvertisedAddress netip.Addr        `yaml:"advertisedAddress" protobuf:"2"`
	ListenAddress     netip.Addr        `yaml:"listenAddress" protobuf:"5"`
	Image             string            `yaml:"image" protobuf:"3"`
	ExtraArgs         map[string]string `yaml:"extraArgs" protobuf:"4"`
}

// NewSpec initializes a Spec resource.
func NewSpec(namespace resource.Namespace, id resource.ID) *Spec {
	return typed.NewResource[SpecSpec, SpecRD](
		resource.NewMetadata(namespace, SpecType, id, resource.VersionUndefined),
		SpecSpec{},
	)
}

// SpecRD provides auxiliary methods for Spec.
type SpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (SpecRD) ResourceDefinition(resource.Metadata, SpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: "{.name}",
			},
			{
				Name:     "AdvertisedAddress",
				JSONPath: "{.advertisedAddress}",
			},
		},
	}
}
