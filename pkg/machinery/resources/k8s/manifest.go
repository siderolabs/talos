// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ManifestType is type of Manifest resource.
const ManifestType = resource.Type("Manifests.kubernetes.talos.dev")

// Manifest resource holds definition of kubelet static pod.
type Manifest = typed.Resource[ManifestSpec, ManifestExtension]

// ManifestSpec holds the Kubernetes resources spec.
//
//gotagsrewrite:gen
type ManifestSpec struct {
	Items []SingleManifest `protobuf:"1" yaml:"items"`
}

// SingleManifest is a single manifest.
//
//gotagsrewrite:gen
type SingleManifest struct {
	Object map[string]any `protobuf:"1" yaml:",inline"`
}

// MarshalYAML implements yaml.Marshaler.
func (spec ManifestSpec) MarshalYAML() (any, error) {
	return spec.Items, nil
}

// NewManifest initializes an empty Manifest resource.
func NewManifest(namespace resource.Namespace, id resource.ID) *Manifest {
	return typed.NewResource[ManifestSpec, ManifestExtension](
		resource.NewMetadata(namespace, ManifestType, id, resource.VersionUndefined),
		ManifestSpec{},
	)
}

// ManifestExtension provides auxiliary methods for Manifest.
type ManifestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ManifestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ManifestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ManifestSpec](ManifestType, &Manifest{})
	if err != nil {
		panic(err)
	}
}
