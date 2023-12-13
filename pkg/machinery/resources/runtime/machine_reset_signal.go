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

// MachineResetSignalType is type of MachineResetSignal resource.
const MachineResetSignalType = resource.Type("MachineResetSignals.runtime.talos.dev")

// MachineResetSignalID is singleton MachineResetSignal resource ID.
const MachineResetSignalID = resource.ID("machine")

// MachineResetSignal resource is created to signal that the machine is going to be reset soon.
//
// This resource is created when all remaining actions are local to the node, and network communication is not required.
type MachineResetSignal = typed.Resource[MachineResetSignalSpec, MachineResetSignalExtension]

// MachineResetSignalSpec describes the spec of MachineResetSignal.
//
//gotagsrewrite:gen
type MachineResetSignalSpec struct{}

// NewMachineResetSignal initializes a MachineResetSignal resource.
func NewMachineResetSignal() *MachineResetSignal {
	return typed.NewResource[MachineResetSignalSpec, MachineResetSignalExtension](
		resource.NewMetadata(NamespaceName, MachineResetSignalType, MachineResetSignalID, resource.VersionUndefined),
		MachineResetSignalSpec{},
	)
}

// MachineResetSignalExtension is auxiliary resource data for MachineResetSignal.
type MachineResetSignalExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MachineResetSignalExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineResetSignalType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MachineResetSignalSpec](MachineResetSignalType, &MachineResetSignal{})
	if err != nil {
		panic(err)
	}
}
