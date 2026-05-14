// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LVMRefreshStatusType is the type of LVMRefreshStatus resource.
const LVMRefreshStatusType = resource.Type("LVMRefreshStatuses.storage.talos.dev")

// LVMRefreshStatus reports the highest LVMRefreshRequest.Request value the
// scan controller has processed. Consumers can compare it to the request
// counter to know whether their requested refresh has completed.
type LVMRefreshStatus = typed.Resource[LVMRefreshStatusSpec, LVMRefreshStatusExtension]

// LVMRefreshStatusSpec is the spec for LVMRefreshStatus.
//
//gotagsrewrite:gen
type LVMRefreshStatusSpec struct {
	Request int `yaml:"request" protobuf:"1"`
}

// NewLVMRefreshStatus initializes a LVMRefreshStatus resource.
func NewLVMRefreshStatus(namespace resource.Namespace, id resource.ID) *LVMRefreshStatus {
	return typed.NewResource[LVMRefreshStatusSpec, LVMRefreshStatusExtension](
		resource.NewMetadata(namespace, LVMRefreshStatusType, id, resource.VersionUndefined),
		LVMRefreshStatusSpec{},
	)
}

// LVMRefreshStatusExtension is auxiliary resource data for LVMRefreshStatus.
type LVMRefreshStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMRefreshStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMRefreshStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Request", JSONPath: "{.request}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMRefreshStatusType, &LVMRefreshStatus{}); err != nil {
		panic(err)
	}
}
