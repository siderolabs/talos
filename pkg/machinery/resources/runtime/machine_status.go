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

// MachineStatusType is type of MachineStatus resource.
const MachineStatusType = resource.Type("MachineStatuses.runtime.talos.dev")

// MachineStatusID is singleton MachineStatus resource ID.
const MachineStatusID = resource.ID("machine")

// MachineStatus resource holds information about aggregated machine status.
type MachineStatus = typed.Resource[MachineStatusSpec, MachineStatusRD]

// MachineStatusSpec describes status of the defined sysctls.
//
//gotagsrewrite:gen
type MachineStatusSpec struct {
	Stage  MachineStage        `yaml:"stage" protobuf:"1"`
	Status MachineStatusStatus `yaml:"status" protobuf:"2"`
}

// MachineStatusStatus describes machine current status at the stage.
//
//gotagsrewrite:gen
type MachineStatusStatus struct {
	Ready           bool             `yaml:"ready" protobuf:"1"`
	UnmetConditions []UnmetCondition `yaml:"unmetConditions" protobuf:"2"`
}

// UnmetCondition is a failure which prevents machine from being ready at the stage.
//
//gotagsrewrite:gen
type UnmetCondition struct {
	Name   string `yaml:"name" protobuf:"1"`
	Reason string `yaml:"reason" protobuf:"2"`
}

// NewMachineStatus initializes a MachineStatus resource.
func NewMachineStatus() *MachineStatus {
	return typed.NewResource[MachineStatusSpec, MachineStatusRD](
		resource.NewMetadata(NamespaceName, MachineStatusType, MachineStatusID, resource.VersionUndefined),
		MachineStatusSpec{},
	)
}

// MachineStatusRD is auxiliary resource data for MachineStatus.
type MachineStatusRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MachineStatusRD) ResourceDefinition(resource.Metadata, MachineStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Stage",
				JSONPath: `{.stage}`,
			},
			{
				Name:     "Ready",
				JSONPath: `{.status.ready}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MachineStatusSpec](MachineStatusType, &MachineStatus{})
	if err != nil {
		panic(err)
	}
}
