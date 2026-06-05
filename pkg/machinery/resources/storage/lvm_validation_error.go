// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage //nolint:dupl

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LVMValidationErrorType is the type of LVMValidationError resource.
const LVMValidationErrorType = resource.Type("LVMValidationErrors.storage.talos.dev")

// LVMValidationError surfaces a problem with the declarative LVM configuration
// that the controllers cannot resolve, e.g. a disk claimed by two volume
// groups. The resource ID identifies the offending volume group.
type LVMValidationError = typed.Resource[LVMValidationErrorSpec, LVMValidationErrorExtension]

// LVMValidationErrorSpec is the spec for LVMValidationError resource.
//
//gotagsrewrite:gen
type LVMValidationErrorSpec struct {
	// VGName is the volume group the error relates to.
	VGName string `yaml:"vgName" protobuf:"1"`
	// Message describes the validation error.
	Message string `yaml:"message" protobuf:"2"`
}

// NewLVMValidationError initializes a LVMValidationError resource.
func NewLVMValidationError(namespace resource.Namespace, id resource.ID) *LVMValidationError {
	return typed.NewResource[LVMValidationErrorSpec, LVMValidationErrorExtension](
		resource.NewMetadata(namespace, LVMValidationErrorType, id, resource.VersionUndefined),
		LVMValidationErrorSpec{},
	)
}

// LVMValidationErrorExtension is auxiliary resource data for LVMValidationError.
type LVMValidationErrorExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMValidationErrorExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMValidationErrorType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "VG", JSONPath: "{.vgName}"}, //nolint:goconst
			{Name: "Message", JSONPath: "{.message}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMValidationErrorType, &LVMValidationError{}); err != nil {
		panic(err)
	}
}
