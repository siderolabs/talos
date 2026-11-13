// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// BootstrapRequestType is type of BootstrapRequest resource.
const BootstrapRequestType = resource.Type("EtcdBootstrapRequests.etcd.talos.dev")

// BootstrapRequestID is the ID of the singleton BootstrapRequest resource.
const BootstrapRequestID = resource.ID("bootstrap")

// BootstrapRequest is a request to bootstrap the etcd cluster on this machine.
//
// The resource is created by the Bootstrap API, and it is picked up by the etcd membership controller:
// the etcd member on this machine is started as a new single-member cluster instead of joining an existing one.
//
// The request is only honored while the machine is not an etcd member yet (the etcd data directory is empty).
type BootstrapRequest = typed.Resource[BootstrapRequestSpec, BootstrapRequestExtension]

// BootstrapRequestSpec describes the etcd bootstrap request.
//
//gotagsrewrite:gen
type BootstrapRequestSpec struct {
	// RecoverFromSnapshot requests to recover the etcd data from the previously uploaded snapshot.
	RecoverFromSnapshot bool `yaml:"recoverFromSnapshot" protobuf:"1"`
	// RecoverSkipHashCheck skips the integrity check of the snapshot on recovery.
	RecoverSkipHashCheck bool `yaml:"recoverSkipHashCheck" protobuf:"2"`
}

// NewBootstrapRequest initializes a BootstrapRequest resource.
func NewBootstrapRequest() *BootstrapRequest {
	return typed.NewResource[BootstrapRequestSpec, BootstrapRequestExtension](
		resource.NewMetadata(NamespaceName, BootstrapRequestType, BootstrapRequestID, resource.VersionUndefined),
		BootstrapRequestSpec{},
	)
}

// BootstrapRequestExtension provides auxiliary methods for BootstrapRequest.
type BootstrapRequestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BootstrapRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BootstrapRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Recover From Snapshot",
				JSONPath: "{.recoverFromSnapshot}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[BootstrapRequestSpec](BootstrapRequestType, &BootstrapRequest{})
	if err != nil {
		panic(err)
	}
}
