// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EtcFileStatusType is type of EtcFile resource.
const EtcFileStatusType = resource.Type("EtcFileStatuses.files.talos.dev")

// EtcFileStatus resource holds contents of the file which should be put to `/etc` directory.
type EtcFileStatus = typed.Resource[EtcFileStatusSpec, EtcFileStatusMD]

// EtcFileStatusSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type EtcFileStatusSpec struct {
	SpecVersion string `yaml:"specVersion" protobuf:"1"`
}

// NewEtcFileStatus initializes a EtcFileStatus resource.
func NewEtcFileStatus(namespace resource.Namespace, id resource.ID) *EtcFileStatus {
	return typed.NewResource[EtcFileStatusSpec, EtcFileStatusMD](
		resource.NewMetadata(namespace, EtcFileStatusType, id, resource.VersionUndefined),
		EtcFileStatusSpec{},
	)
}

// EtcFileStatusMD provides auxiliary methods for EtcFileStatus.
type EtcFileStatusMD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (EtcFileStatusMD) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcFileStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EtcFileStatusSpec](EtcFileStatusType, &EtcFileStatus{})
	if err != nil {
		panic(err)
	}
}
