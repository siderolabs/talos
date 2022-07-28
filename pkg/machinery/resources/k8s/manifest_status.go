// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// ManifestStatusType is type of ManifestStatus resource.
const ManifestStatusType = resource.Type("ManifestStatuses.kubernetes.talos.dev")

// ManifestStatusID is a singleton resource ID.
const ManifestStatusID = resource.ID("manifests")

// ManifestStatus resource holds definition of kubelet static pod.
type ManifestStatus = typed.Resource[ManifestStatusSpec, ManifestStatusRD]

// ManifestStatusSpec describes manifest application status.
//gotagsrewrite:gen
type ManifestStatusSpec struct {
	ManifestsApplied []string `yaml:"manifestsApplied" protobuf:"1"`
}

// NewManifestStatus initializes an empty ManifestStatus resource.
func NewManifestStatus(namespace resource.Namespace) *ManifestStatus {
	return typed.NewResource[ManifestStatusSpec, ManifestStatusRD](
		resource.NewMetadata(namespace, ManifestStatusType, ManifestStatusID, resource.VersionUndefined),
		ManifestStatusSpec{},
	)
}

// ManifestStatusRD provides auxiliary methods for ManifestStatus.
type ManifestStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (ManifestStatusRD) ResourceDefinition(resource.Metadata, ManifestStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ManifestStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
