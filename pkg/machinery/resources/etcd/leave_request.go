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

// LeaveRequestType is type of LeaveRequest resource.
const LeaveRequestType = resource.Type("EtcdLeaveRequests.etcd.talos.dev")

// LeaveRequestID is the ID of the singleton LeaveRequest resource.
const LeaveRequestID = resource.ID("leave")

// LeaveRequest is a request for the etcd member on this machine to leave the etcd cluster.
//
// The resource is created by the EtcdLeaveCluster API (and by the machine reset), and it is picked up
// by the etcd membership controller: the member is removed from the etcd cluster, etcd is stopped,
// and the etcd data is wiped.
//
// While the resource exists, the machine does not attempt to join the etcd cluster again:
// the resource is not persisted, so the machine joins the cluster back after a reboot.
type LeaveRequest = typed.Resource[LeaveRequestSpec, LeaveRequestExtension]

// LeaveRequestSpec is empty.
type LeaveRequestSpec struct{}

// NewLeaveRequest initializes a LeaveRequest resource.
func NewLeaveRequest() *LeaveRequest {
	return typed.NewResource[LeaveRequestSpec, LeaveRequestExtension](
		resource.NewMetadata(NamespaceName, LeaveRequestType, LeaveRequestID, resource.VersionUndefined),
		LeaveRequestSpec{},
	)
}

// LeaveRequestExtension provides auxiliary methods for LeaveRequest.
type LeaveRequestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LeaveRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LeaveRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LeaveRequestSpec](LeaveRequestType, &LeaveRequest{})
	if err != nil {
		panic(err)
	}
}
