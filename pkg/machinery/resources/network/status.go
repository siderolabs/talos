// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// StatusType is type of Status resource.
const StatusType = resource.Type("NetworkStatuses.net.talos.dev")

// Status resource holds status of networking setup.
type Status = typed.Resource[StatusSpec, StatusRD]

// StatusSpec describes network state.
type StatusSpec struct {
	AddressReady      bool `yaml:"addressReady"`
	ConnectivityReady bool `yaml:"connectivityReady"`
	HostnameReady     bool `yaml:"hostnameReady"`
	EtcFilesReady     bool `yaml:"etcFilesReady"`
}

// DeepCopy generates a deep copy of StatusSpec.
func (spec StatusSpec) DeepCopy() StatusSpec {
	return spec
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
func (StatusRD) ResourceDefinition(resource.Metadata, StatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StatusType,
		Aliases:          []resource.Type{"netstatus", "netstatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
