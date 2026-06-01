// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1 //nolint:dupl

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// AcquireConfigStatusType is type of AcquireConfigStatus resource.
const AcquireConfigStatusType = resource.Type("AcquireConfigStatuses.v1alpha1.talos.dev")

// AcquireConfigStatus is created when machine configuration is ready and boot process is ok to proceed.
type AcquireConfigStatus = typed.Resource[AcquireConfigStatusSpec, AcquireConfigStatusExtension]

// AcquireConfigStatusSpec describe state of ready proceed booting with machine config.
//
//gotagsrewrite:gen
type AcquireConfigStatusSpec struct{}

// NewAcquireConfigStatus initializes a AcquireConfigStatus resource.
func NewAcquireConfigStatus() *AcquireConfigStatus {
	return typed.NewResource[AcquireConfigStatusSpec, AcquireConfigStatusExtension](
		resource.NewMetadata(NamespaceName, AcquireConfigStatusType, "machine-config", resource.VersionUndefined),
		AcquireConfigStatusSpec{},
	)
}

// AcquireConfigStatusExtension provides auxiliary methods for AcquireConfigStatus.
type AcquireConfigStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AcquireConfigStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AcquireConfigStatusType,
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AcquireConfigStatusSpec](AcquireConfigStatusType, &AcquireConfigStatus{})
	if err != nil {
		panic(err)
	}
}
