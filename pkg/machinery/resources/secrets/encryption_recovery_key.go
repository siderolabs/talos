// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets //nolint:dupl

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EncryptionRecoveryKeyType is type of EncryptionRecoveryKey resource.
const EncryptionRecoveryKeyType = resource.Type("EncryptionRecoveryKeys.secrets.talos.dev")

// EncryptionRecoveryKey holds a disk encryption recovery key supplied by the operator for a volume.
//
// The resource ID is the volume ID. The resource is created via the API, and destroyed by the
// volume manager once the key was used to unlock the volume or to enroll the recovery key slot.
type EncryptionRecoveryKey = typed.Resource[EncryptionRecoveryKeySpec, EncryptionRecoveryKeyExtension]

// EncryptionRecoveryKeySpec describes the recovery key.
//
//gotagsrewrite:gen
type EncryptionRecoveryKeySpec struct {
	Key []byte `yaml:"key" protobuf:"1"`
}

// NewEncryptionRecoveryKey initializes a EncryptionRecoveryKey resource.
func NewEncryptionRecoveryKey(volumeID resource.ID) *EncryptionRecoveryKey {
	return typed.NewResource[EncryptionRecoveryKeySpec, EncryptionRecoveryKeyExtension](
		resource.NewMetadata(NamespaceName, EncryptionRecoveryKeyType, volumeID, resource.VersionUndefined),
		EncryptionRecoveryKeySpec{},
	)
}

// EncryptionRecoveryKeyExtension provides auxiliary methods for EncryptionRecoveryKey.
type EncryptionRecoveryKeyExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (EncryptionRecoveryKeyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EncryptionRecoveryKeyType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EncryptionRecoveryKeySpec](EncryptionRecoveryKeyType, &EncryptionRecoveryKey{})
	if err != nil {
		panic(err)
	}
}
