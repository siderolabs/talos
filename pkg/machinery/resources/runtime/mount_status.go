// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// MountStatusType is type of Mount resource.
const MountStatusType = resource.Type("MountStatuses.runtime.talos.dev")

// MountStatus resource holds defined sysctl flags status.
type MountStatus struct {
	md   resource.Metadata
	spec MountStatusSpec
}

// MountStatusSpec describes status of the defined sysctls.
type MountStatusSpec struct {
	Source         string   `yaml:"source"`
	Target         string   `yaml:"target"`
	FilesystemType string   `yaml:"filesystemType"`
	Options        []string `yaml:"options"`
}

// NewMountStatus initializes a MountStatus resource.
func NewMountStatus(namespace resource.Namespace, id resource.ID) *MountStatus {
	r := &MountStatus{
		md:   resource.NewMetadata(namespace, MountStatusType, id, resource.VersionUndefined),
		spec: MountStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *MountStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *MountStatus) Spec() interface{} {
	return r.spec
}

func (r *MountStatus) String() string {
	return fmt.Sprintf("runtime.MountStatus.(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *MountStatus) DeepCopy() resource.Resource {
	return &MountStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *MountStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MountStatusType,
		Aliases:          []resource.Type{"mounts"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Source",
				JSONPath: `{.source}`,
			},
			{
				Name:     "Target",
				JSONPath: `{.target}`,
			},
			{
				Name:     "Filesystem Type",
				JSONPath: `{.filesystemType}`,
			},
		},
	}
}

// TypedSpec allows to access the MountStatusSpec with the proper type.
func (r *MountStatus) TypedSpec() *MountStatusSpec {
	return &r.spec
}
