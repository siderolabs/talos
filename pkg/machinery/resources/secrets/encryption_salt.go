// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EncryptionSaltType is type of EncryptionSalt resource.
const EncryptionSaltType = resource.Type("EncryptionSalts.secrets.talos.dev")

// EncryptionSaltID is a resource ID of singleton instance.
const EncryptionSaltID = resource.ID("salt")

// EncryptionSalt contains salt data used to mix in into disk encryption keys.
type EncryptionSalt = typed.Resource[EncryptionSaltSpec, EncryptionSaltExtension]

// EncryptionSaltSpec describes the salt.
//
//gotagsrewrite:gen
type EncryptionSaltSpec struct {
	DiskSalt []byte `yaml:"diskSalt" protobuf:"1"`
}

// NewEncryptionSalt initializes a EncryptionSalt resource.
func NewEncryptionSalt() *EncryptionSalt {
	return typed.NewResource[EncryptionSaltSpec, EncryptionSaltExtension](
		resource.NewMetadata(NamespaceName, EncryptionSaltType, EncryptionSaltID, resource.VersionUndefined),
		EncryptionSaltSpec{},
	)
}

// EncryptionSaltExtension provides auxiliary methods for EncryptionSalt.
type EncryptionSaltExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (EncryptionSaltExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EncryptionSaltType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[EncryptionSaltSpec](EncryptionSaltType, &EncryptionSalt{}); err != nil {
		panic(err)
	}
}
