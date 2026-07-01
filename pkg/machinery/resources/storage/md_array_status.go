// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MDArrayStatusType is the type of MDArrayStatus resource.
const MDArrayStatusType = resource.Type("MDArrayStatuses.storage.talos.dev")

// MDArrayStatus is the observed state of an MD (software RAID) array.
type MDArrayStatus = typed.Resource[MDArrayStatusSpec, MDArrayStatusExtension]

// MDArrayStatusSpec is the spec for MDArrayStatus resource.
//
//gotagsrewrite:gen
type MDArrayStatusSpec struct {
	// Name is the array name.
	Name string `yaml:"name" protobuf:"1"`
	// Level is the RAID level.
	Level MDLevel `yaml:"level" protobuf:"2"`
	// Device is the stable by-id device path of the array.
	Device string `yaml:"device" protobuf:"3"`
	// Members is the list of member device paths.
	Members []string `yaml:"members" protobuf:"4"`
}

// NewMDArrayStatus initializes an MDArrayStatus resource.
func NewMDArrayStatus(namespace resource.Namespace, id resource.ID) *MDArrayStatus {
	return typed.NewResource[MDArrayStatusSpec, MDArrayStatusExtension](
		resource.NewMetadata(namespace, MDArrayStatusType, id, resource.VersionUndefined),
		MDArrayStatusSpec{},
	)
}

// MDArrayStatusExtension is auxiliary resource data for MDArrayStatus.
type MDArrayStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MDArrayStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MDArrayStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Level", JSONPath: "{.level}"},
			{Name: "Device", JSONPath: "{.device}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(MDArrayStatusType, &MDArrayStatus{}); err != nil {
		panic(err)
	}
}
