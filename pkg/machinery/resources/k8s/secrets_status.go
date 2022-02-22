// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
//
//nolint:dupl
package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// SecretsStatusType is type of SecretsStatus resource.
const SecretsStatusType = resource.Type("SecretStatuses.kubernetes.talos.dev")

// StaticPodSecretsStaticPodID is resource ID for SecretStatus resource for static pods.
const StaticPodSecretsStaticPodID = resource.ID("static-pods")

// SecretsStatus resource holds definition of rendered secrets.
type SecretsStatus struct {
	md   resource.Metadata
	spec SecretsStatusSpec
}

// SecretsStatusSpec describes status of rendered secrets.
type SecretsStatusSpec struct {
	Ready   bool   `yaml:"ready"`
	Version string `yaml:"version"`
}

// NewSecretsStatus initializes a SecretsStatus resource.
func NewSecretsStatus(namespace resource.Namespace, id resource.ID) *SecretsStatus {
	r := &SecretsStatus{
		md:   resource.NewMetadata(namespace, SecretsStatusType, id, resource.VersionUndefined),
		spec: SecretsStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *SecretsStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *SecretsStatus) Spec() interface{} {
	return r.spec
}

func (r *SecretsStatus) String() string {
	return fmt.Sprintf("k8s.SecretStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *SecretsStatus) DeepCopy() resource.Resource {
	return &SecretsStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *SecretsStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SecretsStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: "{.ready}",
			},
			{
				Name:     "Secrets Version",
				JSONPath: "{.version}",
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *SecretsStatus) TypedSpec() *SecretsStatusSpec {
	return &r.spec
}
