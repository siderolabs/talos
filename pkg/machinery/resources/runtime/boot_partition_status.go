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

// BootPartitionStatusType is type of BootPartitionStatus resource.
const BootPartitionStatusType = resource.Type("BootPartitionStatuses.runtime.talos.dev")

// BootPartitionStatus resource describes the partition the machine was booted from.
//
// The resource only exists if the boot partition is known: for sd-boot it comes from the
// `LoaderDevicePartUUID` EFI variable (the ESP), for GRUB from the `talos.boot.partuuid` kernel
// argument set by the generated GRUB config (the BOOT partition). On kexec, Talos passes the
// `talos.boot.partuuid` kernel argument, which takes precedence over the EFI variable.
type BootPartitionStatus = typed.Resource[BootPartitionStatusSpec, BootPartitionStatusExtension]

// BootPartitionStatusID is the ID of BootPartitionStatus resource.
const BootPartitionStatusID = resource.ID("boot-partition")

// BootPartitionStatusSpec describes the partition the machine was booted from.
//
//gotagsrewrite:gen
type BootPartitionStatusSpec struct {
	// PartitionUUID is the GPT partition UUID of the boot partition.
	PartitionUUID string `yaml:"partitionUUID" protobuf:"1"`
}

// NewBootPartitionStatus initializes a BootPartitionStatus resource.
func NewBootPartitionStatus(namespace resource.Namespace, id resource.ID) *BootPartitionStatus {
	return typed.NewResource[BootPartitionStatusSpec, BootPartitionStatusExtension](
		resource.NewMetadata(namespace, BootPartitionStatusType, id, resource.VersionUndefined),
		BootPartitionStatusSpec{},
	)
}

// BootPartitionStatusExtension is auxiliary resource data for BootPartitionStatus.
type BootPartitionStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (BootPartitionStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BootPartitionStatusType,
		Aliases:          []resource.Type{"bootpartition"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Partition UUID",
				JSONPath: `{.partitionUUID}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[BootPartitionStatusSpec](BootPartitionStatusType, &BootPartitionStatus{})
	if err != nil {
		panic(err)
	}
}
