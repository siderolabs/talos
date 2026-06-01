// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// InfoType is type of Info resource.
const InfoType = resource.Type("Infos.cluster.talos.dev")

// InfoID is the resource ID for the current cluster info.
const InfoID = resource.ID("current")

// Info resource holds cluster information.
type Info = typed.Resource[InfoSpec, InfoExtension]

// InfoSpec describes cluster information.
//
//gotagsrewrite:gen
type InfoSpec struct {
	ClusterID   string `yaml:"clusterId" protobuf:"1"`
	ClusterName string `yaml:"clusterName" protobuf:"2"`
}

// NewInfo initializes an Info resource.
func NewInfo() *Info {
	return typed.NewResource[InfoSpec, InfoExtension](
		resource.NewMetadata(NamespaceName, InfoType, InfoID, resource.VersionUndefined),
		InfoSpec{},
	)
}

// InfoExtension provides auxiliary methods for Info.
type InfoExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InfoExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfoType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Cluster ID",
				JSONPath: `{.clusterId}`,
			},
			{
				Name:     "Cluster Name",
				JSONPath: `{.clusterName}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[InfoSpec](InfoType, &Info{})
	if err != nil {
		panic(err)
	}
}
