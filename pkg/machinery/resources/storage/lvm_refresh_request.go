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

// LVMRefreshRequestType is the type of LVMRefreshRequest resource.
const LVMRefreshRequestType = resource.Type("LVMRefreshRequests.storage.talos.dev")

// RefreshID is the ID of the singleton LVM refresh request/status pair.
const RefreshID resource.ID = "refresh"

// LVMRefreshRequest signals the LVM scan controller to re-run vgs/pvs/lvs.
//
// Mirrors the block.DiscoveryRefreshRequest pattern: a monotonically
// increasing counter is bumped by event sources (trigger controller), and
// the scan controller echoes the observed value in LVMRefreshStatus once
// the corresponding scan has completed.
type LVMRefreshRequest = typed.Resource[LVMRefreshRequestSpec, LVMRefreshRequestExtension]

// LVMRefreshRequestSpec is the spec for LVMRefreshRequest.
//
//gotagsrewrite:gen
type LVMRefreshRequestSpec struct {
	Request int `yaml:"request" protobuf:"1"`
}

// NewLVMRefreshRequest initializes a LVMRefreshRequest resource.
func NewLVMRefreshRequest(namespace resource.Namespace, id resource.ID) *LVMRefreshRequest {
	return typed.NewResource[LVMRefreshRequestSpec, LVMRefreshRequestExtension](
		resource.NewMetadata(namespace, LVMRefreshRequestType, id, resource.VersionUndefined),
		LVMRefreshRequestSpec{},
	)
}

// LVMRefreshRequestExtension is auxiliary resource data for LVMRefreshRequest.
type LVMRefreshRequestExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMRefreshRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMRefreshRequestType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Request", JSONPath: "{.request}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMRefreshRequestType, &LVMRefreshRequest{}); err != nil {
		panic(err)
	}
}
