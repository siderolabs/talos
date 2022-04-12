// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// ManifestType is type of Manifest resource.
const ManifestType = resource.Type("Manifests.kubernetes.talos.dev")

// Manifest resource holds definition of kubelet static pod.
type Manifest struct {
	md   resource.Metadata
	spec *ManifestSpec
}

// ManifestSpec holds the Kubernetes resources spec.
type ManifestSpec struct {
	Items []map[string]interface{}
}

// MarshalYAML implements yaml.Marshaler.
func (spec *ManifestSpec) MarshalYAML() (interface{}, error) {
	return spec.Items, nil
}

// NewManifest initializes an empty Manifest resource.
func NewManifest(namespace resource.Namespace, id resource.ID) *Manifest {
	r := &Manifest{
		md:   resource.NewMetadata(namespace, ManifestType, id, resource.VersionUndefined),
		spec: &ManifestSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Manifest) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Manifest) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *Manifest) DeepCopy() resource.Resource {
	return &Manifest{
		md: r.md,
		spec: &ManifestSpec{
			Items: append([]map[string]interface{}(nil), r.spec.Items...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Manifest) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ManifestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

// TypedSpec returns .spec.
func (r *Manifest) TypedSpec() *ManifestSpec {
	return r.spec
}
