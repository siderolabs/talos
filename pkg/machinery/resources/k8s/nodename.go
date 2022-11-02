// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// NodenameType is type of Nodename resource.
const NodenameType = resource.Type("Nodenames.kubernetes.talos.dev")

// NodenameID is a singleton resource ID for Nodename.
const NodenameID = resource.ID("nodename")

// Nodename resource holds Kubernetes nodename.
type Nodename = typed.Resource[NodenameSpec, NodenameRD]

// NodenameSpec describes Kubernetes nodename.
//
//gotagsrewrite:gen
type NodenameSpec struct {
	Nodename        string `yaml:"nodename" protobuf:"1"`
	HostnameVersion string `yaml:"hostnameVersion" protobuf:"2"`
}

// NewNodename initializes a Nodename resource.
func NewNodename(namespace resource.Namespace, id resource.ID) *Nodename {
	return typed.NewResource[NodenameSpec, NodenameRD](
		resource.NewMetadata(namespace, NodenameType, id, resource.VersionUndefined),
		NodenameSpec{},
	)
}

// NodenameRD provides auxiliary methods for Nodename.
type NodenameRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (NodenameRD) ResourceDefinition(resource.Metadata, NodenameSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodenameType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Nodename",
				JSONPath: "{.nodename}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodenameSpec](NodenameType, &Nodename{})
	if err != nil {
		panic(err)
	}
}
