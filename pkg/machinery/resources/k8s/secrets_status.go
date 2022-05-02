// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
//
//nolint:dupl
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// SecretsStatusType is type of SecretsStatus resource.
const SecretsStatusType = resource.Type("SecretStatuses.kubernetes.talos.dev")

// StaticPodSecretsStaticPodID is resource ID for SecretStatus resource for static pods.
const StaticPodSecretsStaticPodID = resource.ID("static-pods")

// SecretsStatus resource holds definition of rendered secrets.
type SecretsStatus = typed.Resource[SecretsStatusSpec, SecretsStatusRD]

// SecretsStatusSpec describes status of rendered secrets.
type SecretsStatusSpec struct {
	Ready   bool   `yaml:"ready"`
	Version string `yaml:"version"`
}

// DeepCopy implements typed.DeepCopyable interface.
func (spec SecretsStatusSpec) DeepCopy() SecretsStatusSpec { return spec }

// NewSecretsStatus initializes a SecretsStatus resource.
func NewSecretsStatus(namespace resource.Namespace, id resource.ID) *SecretsStatus {
	return typed.NewResource[SecretsStatusSpec, SecretsStatusRD](
		resource.NewMetadata(namespace, SecretsStatusType, id, resource.VersionUndefined),
		SecretsStatusSpec{},
	)
}

// SecretsStatusRD provides auxiliary methods for SecretsStatus.
type SecretsStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (SecretsStatusRD) ResourceDefinition(resource.Metadata, SecretsStatusSpec) meta.ResourceDefinitionSpec {
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
