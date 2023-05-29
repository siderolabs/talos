// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// DevicesStatusType is type of DevicesStatus resource.
const DevicesStatusType = resource.Type("DevicesStatuses.runtime.talos.dev")

// DevicesStatus resource holds status of hardware devices (overall).
type DevicesStatus = typed.Resource[DevicesStatusSpec, DevicesStatusExtension]

// DevicesID is the ID of DevicesStatus resource.
const DevicesID = resource.ID("devices")

// DevicesStatusSpec is the spec for devices status.
//
//gotagsrewrite:gen
type DevicesStatusSpec struct {
	// Devices are settled down and ready to be used.
	Ready bool `yaml:"ready" protobuf:"1"`
}

// NewDevicesStatus initializes a DevicesStatus resource.
func NewDevicesStatus(namespace resource.Namespace, id resource.ID) *DevicesStatus {
	return typed.NewResource[DevicesStatusSpec, DevicesStatusExtension](
		resource.NewMetadata(namespace, DevicesStatusType, id, resource.VersionUndefined),
		DevicesStatusSpec{},
	)
}

// DevicesStatusExtension is auxiliary resource data for DevicesStatus.
type DevicesStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DevicesStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DevicesStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: `{.ready}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[DevicesStatusSpec](DevicesStatusType, &DevicesStatus{})
	if err != nil {
		panic(err)
	}
}
