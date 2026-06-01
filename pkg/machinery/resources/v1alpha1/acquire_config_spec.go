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

// AcquireConfigSpecType is type of AcquireConfigSpec resource.
const AcquireConfigSpecType = resource.Type("AcquireConfigSpecs.v1alpha1.talos.dev")

// AcquireConfigSpec is created when Talos is ready to start acquiring machine configuration.
type AcquireConfigSpec = typed.Resource[AcquireConfigSpecSpec, AcquireConfigSpecExtension]

// AcquireConfigSpecSpec describe state of ready to acquire config.
//
//gotagsrewrite:gen
type AcquireConfigSpecSpec struct{}

// NewAcquireConfigSpec initializes a AcquireConfigSpec resource.
func NewAcquireConfigSpec() *AcquireConfigSpec {
	return typed.NewResource[AcquireConfigSpecSpec, AcquireConfigSpecExtension](
		resource.NewMetadata(NamespaceName, AcquireConfigSpecType, "machine-config", resource.VersionUndefined),
		AcquireConfigSpecSpec{},
	)
}

// AcquireConfigSpecExtension provides auxiliary methods for AcquireConfigSpec.
type AcquireConfigSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AcquireConfigSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AcquireConfigSpecType,
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AcquireConfigSpecSpec](AcquireConfigSpecType, &AcquireConfigSpec{})
	if err != nil {
		panic(err)
	}
}
