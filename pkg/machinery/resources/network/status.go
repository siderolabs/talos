// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// StatusType is type of Status resource.
const StatusType = resource.Type("NetworkStatuses.net.talos.dev")

// Status resource holds status of networking setup.
type Status = typed.Resource[StatusSpec, StatusRD]

// StatusSpec describes network state.
//
//gotagsrewrite:gen
type StatusSpec struct {
	AddressReady      bool `yaml:"addressReady" protobuf:"1"`
	ConnectivityReady bool `yaml:"connectivityReady" protobuf:"2"`
	HostnameReady     bool `yaml:"hostnameReady" protobuf:"3"`
	EtcFilesReady     bool `yaml:"etcFilesReady" protobuf:"4"`
}

// StatusID is the resource ID of the singleton instance.
const StatusID resource.ID = "status"

// NewStatus initializes a Status resource.
func NewStatus(namespace resource.Namespace, id resource.ID) *Status {
	return typed.NewResource[StatusSpec, StatusRD](
		resource.NewMetadata(namespace, StatusType, id, resource.VersionUndefined),
		StatusSpec{},
	)
}

// StatusRD provides auxiliary methods for Status.
type StatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (StatusRD) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StatusType,
		Aliases:          []resource.Type{"netstatus", "netstatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[StatusSpec](StatusType, &Status{})
	if err != nil {
		panic(err)
	}
}
