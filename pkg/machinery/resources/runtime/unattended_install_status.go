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

// UnattendedInstallStatusType is the type of UnattendedInstallStatus resource.
const UnattendedInstallStatusType = resource.Type("UnattendedInstallStatuses.runtime.talos.dev")

// UnattendedInstallStatusID is the singleton ID of the UnattendedInstallStatus resource.
const UnattendedInstallStatusID = resource.ID("unattended-install")

// UnattendedInstallPhase describes the phase of the unattended install.
type UnattendedInstallPhase int

// Unattended install phases.
//
//structprotogen:gen_enum
const (
	UnattendedInstallPhasePending          UnattendedInstallPhase = iota // pending
	UnattendedInstallPhaseInstalling                                     // installing
	UnattendedInstallPhaseInstalled                                      // installed
	UnattendedInstallPhaseWaitingForReboot                               // waiting-for-reboot
	UnattendedInstallPhaseFailed                                         // failed
)

// UnattendedInstallStatus is the status of the unattended install performed by the UnattendedInstallController.
type UnattendedInstallStatus = typed.Resource[UnattendedInstallStatusSpec, UnattendedInstallStatusExtension]

// UnattendedInstallStatusSpec describes the unattended install status.
//
//gotagsrewrite:gen
type UnattendedInstallStatusSpec struct {
	Image string                 `yaml:"image,omitempty" protobuf:"1"`
	Phase UnattendedInstallPhase `yaml:"phase,omitempty" protobuf:"2"`
	Error string                 `yaml:"error,omitempty" protobuf:"3"`
}

// UnattendedInstallStatusExtension provides auxiliary methods for UnattendedInstallStatus resource.
type UnattendedInstallStatusExtension struct{}

// NewUnattendedInstallStatus initializes a new UnattendedInstallStatus resource.
func NewUnattendedInstallStatus() *UnattendedInstallStatus {
	return typed.NewResource[UnattendedInstallStatusSpec, UnattendedInstallStatusExtension](
		resource.NewMetadata(NamespaceName, UnattendedInstallStatusType, UnattendedInstallStatusID, resource.VersionUndefined),
		UnattendedInstallStatusSpec{},
	)
}

// ResourceDefinition implements [typed.Extension] interface.
func (UnattendedInstallStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             UnattendedInstallStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Phase",
				JSONPath: `{.phase}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[UnattendedInstallStatusSpec](UnattendedInstallStatusType, &UnattendedInstallStatus{})
	if err != nil {
		panic(err)
	}
}
