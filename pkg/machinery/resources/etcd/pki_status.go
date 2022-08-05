// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// PKIStatusType is type of PKIStatus resource.
const PKIStatusType = resource.Type("PKIStatuses.etcd.talos.dev")

// PKIID is resource ID for PKIStatus resource for etcd.
const PKIID = resource.ID("etcd")

// PKIStatus resource holds status of rendered secrets.
type PKIStatus = typed.Resource[PKIStatusSpec, PKIStatusRD]

// PKIStatusSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type PKIStatusSpec struct {
	Ready   bool   `yaml:"ready" protobuf:"1"`
	Version string `yaml:"version" protobuf:"2"`
}

// NewPKIStatus initializes a PKIStatus resource.
func NewPKIStatus(namespace resource.Namespace, id resource.ID) *PKIStatus {
	return typed.NewResource[PKIStatusSpec, PKIStatusRD](
		resource.NewMetadata(namespace, PKIStatusType, id, resource.VersionUndefined),
		PKIStatusSpec{},
	)
}

// PKIStatusRD provides auxiliary methods for PKIStatus.
type PKIStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (PKIStatusRD) ResourceDefinition(resource.Metadata, PKIStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PKIStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: "{.ready}",
			},
			{
				Name:     "Secrets Version",
				JSONPath: "{.version}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[PKIStatusSpec](PKIStatusType, &PKIStatus{})
	if err != nil {
		panic(err)
	}
}
