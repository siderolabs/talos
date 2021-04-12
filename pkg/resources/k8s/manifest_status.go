// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// ManifestStatusType is type of ManifestStatus resource.
const ManifestStatusType = resource.Type("ManifestStatuses.kubernetes.talos.dev")

// ManifestStatusID is a singleton resource ID.
const ManifestStatusID = resource.ID("manifests")

// ManifestStatus resource holds definition of kubelet static pod.
type ManifestStatus struct {
	md   resource.Metadata
	spec ManifestStatusSpec
}

// ManifestStatusSpec describes manifest application status.
type ManifestStatusSpec struct {
	ManifestsApplied []string `yaml:"manifestsApplied"`
}

// NewManifestStatus initializes an empty ManifestStatus resource.
func NewManifestStatus(namespace resource.Namespace) *ManifestStatus {
	r := &ManifestStatus{
		md:   resource.NewMetadata(namespace, ManifestStatusType, ManifestStatusID, resource.VersionUndefined),
		spec: ManifestStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *ManifestStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *ManifestStatus) Spec() interface{} {
	return r.spec
}

func (r *ManifestStatus) String() string {
	return fmt.Sprintf("k8s.ManifestStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *ManifestStatus) DeepCopy() resource.Resource {
	return &ManifestStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// Status returns ManifestStatusSpec.
func (r *ManifestStatus) Status() *ManifestStatusSpec {
	return &r.spec
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *ManifestStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ManifestStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
