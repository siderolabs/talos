// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// EtcFileStatusType is type of EtcFile resource.
const EtcFileStatusType = resource.Type("EtcFileStatuses.files.talos.dev")

// EtcFileStatus resource holds contents of the file which should be put to `/etc` directory.
type EtcFileStatus = typed.Resource[EtcFileStatusSpec, EtcFileStatusMD]

// EtcFileStatusSpec describes status of rendered secrets.
type EtcFileStatusSpec struct {
	SpecVersion string `yaml:"specVersion"`
}

// DeepCopy implements typed.DeepCopyable interface.
func (e EtcFileStatusSpec) DeepCopy() EtcFileStatusSpec {
	return e
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
func (EtcFileStatusMD) ResourceDefinition(resource.Metadata, EtcFileStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcFileStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
