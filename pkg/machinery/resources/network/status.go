// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// StatusType is type of Status resource.
const StatusType = resource.Type("NetworkStatuses.net.talos.dev")

// Status resource holds status of networking setup.
type Status struct {
	md   resource.Metadata
	spec StatusSpec
}

// StatusSpec describes network state.
type StatusSpec struct {
	AddressReady      bool `yaml:"addressReady"`
	ConnectivityReady bool `yaml:"connectivityReady"`
	HostnameReady     bool `yaml:"hostnameReady"`
	EtcFilesReady     bool `yaml:"etcFilesReady"`
}

// StatusID is the resource ID of the singleton instance.
const StatusID resource.ID = "status"

// NewStatus initializes a Status resource.
func NewStatus(namespace resource.Namespace, id resource.ID) *Status {
	r := &Status{
		md:   resource.NewMetadata(namespace, StatusType, id, resource.VersionUndefined),
		spec: StatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Status) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Status) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *Status) DeepCopy() resource.Resource {
	return &Status{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Status) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StatusType,
		Aliases:          []resource.Type{"netstatus", "netstatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *Status) TypedSpec() *StatusSpec {
	return &r.spec
}
