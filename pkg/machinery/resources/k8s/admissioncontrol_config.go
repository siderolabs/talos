// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// AdmissionControlConfigType is type of AdmissionControlConfig resource.
const AdmissionControlConfigType = resource.Type("AdmissionControlConfigs.kubernetes.talos.dev")

// AdmissionControlConfigID is a singleton resource ID for AdmissionControlConfig.
const AdmissionControlConfigID = resource.ID("admission-control")

// AdmissionControlConfig represents configuration for kube-apiserver Admission Control plugins.
type AdmissionControlConfig = typed.Resource[AdmissionControlConfigSpec, AdmissionControlConfigExtension]

// AdmissionControlConfigSpec is configuration for kube-apiserver.
//
//gotagsrewrite:gen
type AdmissionControlConfigSpec struct {
	Config []AdmissionPluginSpec `yaml:"config" protobuf:"1"`
}

// AdmissionPluginSpec is a single admission plugin configuration Admission Control plugins.
//
//gotagsrewrite:gen
type AdmissionPluginSpec struct {
	Name          string         `yaml:"name" protobuf:"1"`
	Configuration map[string]any `yaml:"configuration" protobuf:"2"`
}

// NewAdmissionControlConfig returns new AdmissionControlConfig resource.
func NewAdmissionControlConfig() *AdmissionControlConfig {
	return typed.NewResource[AdmissionControlConfigSpec, AdmissionControlConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, AdmissionControlConfigType, AdmissionControlConfigID, resource.VersionUndefined),
		AdmissionControlConfigSpec{})
}

// AdmissionControlConfigExtension defines AdmissionControlConfig resource definition.
type AdmissionControlConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AdmissionControlConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AdmissionControlConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AdmissionControlConfigSpec](AdmissionControlConfigType, &AdmissionControlConfig{})
	if err != nil {
		panic(err)
	}
}
