// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime //nolint:dupl

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// RebootRequestType is the type of RebootRequest resource.
const RebootRequestType = resource.Type("RebootRequests.runtime.talos.dev")

// RebootRequestID is the singleton RebootRequest resource ID.
const RebootRequestID = resource.ID("reboot-request")

// RebootRequest resource signals that a reboot should be performed.
//
// Controllers that need to trigger a reboot should create (or update) this resource.
// The RebootController watches this resource and performs the actual reboot.
type RebootRequest = typed.Resource[RebootRequestSpec, RebootRequestExtension]

// RebootRequestSpec describes the spec of RebootRequest.
//
//gotagsrewrite:gen
type RebootRequestSpec struct{}

// NewRebootRequest initializes a RebootRequest resource.
func NewRebootRequest() *RebootRequest {
	return typed.NewResource[RebootRequestSpec, RebootRequestExtension](
		resource.NewMetadata(NamespaceName, RebootRequestType, RebootRequestID, resource.VersionUndefined),
		RebootRequestSpec{},
	)
}

// RebootRequestExtension is auxiliary resource data for RebootRequest.
type RebootRequestExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (RebootRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RebootRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[RebootRequestSpec](RebootRequestType, &RebootRequest{})
	if err != nil {
		panic(err)
	}
}
