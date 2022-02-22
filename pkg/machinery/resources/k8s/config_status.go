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

// ConfigStatusType is type of ConfigStatus resource.
const ConfigStatusType = resource.Type("ConfigStatuses.kubernetes.talos.dev")

// ConfigStatusStaticPodID is resource ID for ConfigStatus resource for static pods.
const ConfigStatusStaticPodID = resource.ID("static-pods")

// ConfigStatus resource holds definition of rendered secrets.
type ConfigStatus struct {
	md   resource.Metadata
	spec ConfigStatusSpec
}

// ConfigStatusSpec describes status of rendered secrets.
type ConfigStatusSpec struct {
	Ready   bool   `yaml:"ready"`
	Version string `yaml:"version"`
}

// NewConfigStatus initializes a ConfigStatus resource.
func NewConfigStatus(namespace resource.Namespace, id resource.ID) *ConfigStatus {
	r := &ConfigStatus{
		md:   resource.NewMetadata(namespace, ConfigStatusType, id, resource.VersionUndefined),
		spec: ConfigStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *ConfigStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *ConfigStatus) Spec() interface{} {
	return r.spec
}

func (r *ConfigStatus) String() string {
	return fmt.Sprintf("k8s.ConfigStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *ConfigStatus) DeepCopy() resource.Resource {
	return &ConfigStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *ConfigStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigStatusType,
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
func (r *ConfigStatus) TypedSpec() *ConfigStatusSpec {
	return &r.spec
}
