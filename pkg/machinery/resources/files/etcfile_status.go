// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// EtcFileStatusType is type of EtcFile resource.
const EtcFileStatusType = resource.Type("EtcFileStatuses.files.talos.dev")

// EtcFileStatus resource holds contents of the file which should be put to `/etc` directory.
type EtcFileStatus struct {
	md   resource.Metadata
	spec EtcFileStatusSpec
}

// EtcFileStatusSpec describes status of rendered secrets.
type EtcFileStatusSpec struct {
	SpecVersion string `yaml:"specVersion"`
}

// NewEtcFileStatus initializes a EtcFileStatus resource.
func NewEtcFileStatus(namespace resource.Namespace, id resource.ID) *EtcFileStatus {
	r := &EtcFileStatus{
		md:   resource.NewMetadata(namespace, EtcFileStatusType, id, resource.VersionUndefined),
		spec: EtcFileStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *EtcFileStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *EtcFileStatus) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *EtcFileStatus) DeepCopy() resource.Resource {
	return &EtcFileStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *EtcFileStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcFileStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *EtcFileStatus) TypedSpec() *EtcFileStatusSpec {
	return &r.spec
}
