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

// MDRefreshRequestType is the type of MDRefreshRequest resource.
const MDRefreshRequestType = resource.Type("MDRefreshRequests.storage.talos.dev")

// MDRefreshRequest signals the MD reconcile controller to refresh array status.
type MDRefreshRequest = typed.Resource[MDRefreshRequestSpec, MDRefreshRequestExtension]

// MDRefreshRequestSpec is the spec for MDRefreshRequest.
//
//gotagsrewrite:gen
type MDRefreshRequestSpec struct {
	Request int `yaml:"request" protobuf:"1"`
}

// NewMDRefreshRequest initializes an MDRefreshRequest resource.
func NewMDRefreshRequest(namespace resource.Namespace, id resource.ID) *MDRefreshRequest {
	return typed.NewResource[MDRefreshRequestSpec, MDRefreshRequestExtension](
		resource.NewMetadata(namespace, MDRefreshRequestType, id, resource.VersionUndefined),
		MDRefreshRequestSpec{},
	)
}

// MDRefreshRequestExtension is auxiliary resource data for MDRefreshRequest.
type MDRefreshRequestExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MDRefreshRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MDRefreshRequestType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Request", JSONPath: "{.request}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(MDRefreshRequestType, &MDRefreshRequest{}); err != nil {
		panic(err)
	}
}
