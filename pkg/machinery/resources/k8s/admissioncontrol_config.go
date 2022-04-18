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

// AdmissionControlConfigType is type of AdmissionControlConfig resource.
const AdmissionControlConfigType = resource.Type("AdmissionControlConfigs.kubernetes.talos.dev")

// AdmissionControlConfigID is a singleton resource ID for AdmissionControlConfig.
const AdmissionControlConfigID = resource.ID("admission-control")

// AdmissionControlConfig represents configuration for kube-apiserver Admission Control plugins.
type AdmissionControlConfig = typed.Resource[AdmissionControlConfigSpec, AdmissionControlConfigRD]

// AdmissionControlConfigSpec is configuration for kube-apiserver.
type AdmissionControlConfigSpec struct {
	Config []AdmissionPluginSpec `yaml:"config"`
}

// AdmissionPluginSpec is a single admission plugin configuration Admission Control plugins.
type AdmissionPluginSpec struct {
	Name          string                 `yaml:"name"`
	Configuration map[string]interface{} `yaml:"configuration"`
}

// DeepCopy implements Deepcopyable.
//
// TODO: should be properly go-generated.
func (spec AdmissionControlConfigSpec) DeepCopy() AdmissionControlConfigSpec {
	return spec
}

// NewAdmissionControlConfig returns new AdmissionControlConfig resource.
func NewAdmissionControlConfig() *AdmissionControlConfig {
	return typed.NewResource[AdmissionControlConfigSpec, AdmissionControlConfigRD](
		resource.NewMetadata(ControlPlaneNamespaceName, AdmissionControlConfigType, AdmissionControlConfigID, resource.VersionUndefined),
		AdmissionControlConfigSpec{})
}

// AdmissionControlConfigRD defines AdmissionControlConfig resource definition.
type AdmissionControlConfigRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AdmissionControlConfigRD) ResourceDefinition(_ resource.Metadata, _ AdmissionControlConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AdmissionControlConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
